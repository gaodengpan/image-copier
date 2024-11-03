#! /bin/bash

IMAGE_ID=$1
# 源镜像ID
segs=$(echo "$IMAGE_ID" | awk -F'/' '{print NF}')
if [ ! "$segs" == "3" ]; then
	seg1=$(echo "$IMAGE_ID" | cut -d'/' -f1)
	if [ ! "${seg1##*.}" = "io" ]; then
		IMAGE_ID=docker.io/$IMAGE_ID
	fi
fi
# 构建目标镜像ID
DEST_IMAGE_ID=""
if [ -z "$REGISTRY_NAMESPACE" ]; then
	DEST_IMAGE_ID=$REGISTRY_HOST/$IMAGE_ID
else
	DEST_IMAGE_ID=$REGISTRY_HOST/$REGISTRY_NAMESPACE/$(echo "$IMAGE_ID" | tr '/' '_')
fi

# login
docker login -u "$REGISTRY_USERNAME" -p "$REGISTRY_PASSWD" "$REGISTRY_HOST" >/dev/null 2>&1
if [ $? == 1 ]; then
	echo "login $REGISTRY_HOST failed"
	exit 1
fi
echo "login $REGISTRY_HOST success"
# 尝试拉取目标镜像
docker pull "$DEST_IMAGE_ID" >/dev/null 2>&1
if [ $? == 1 ]; then
	echo "image not found: $DEST_IMAGE_ID"
	# 触发github workflow同步镜像
	echo "dispatch github workflow: image-copier"
	suffix="--$(date '+%s')"
	data=$(printf '{"ref":"master","inputs":{"imageId":"%s","destImageId":"%s","suffix":"%s"}}' "$IMAGE_ID" "$DEST_IMAGE_ID" "$suffix")
	curl -sL \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: Bearer $GITHUB_TOKEN" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/workflows/$GITHUB_WORKFLOW_ID/dispatches \
		-d "$data"

	# 筛选workflow run实例
	runId=""
	while true; do
		runs=$(curl -sL \
			-H "Accept: application/vnd.github+json" \
			-H "Authorization: Bearer $GITHUB_TOKEN" \
			-H "X-GitHub-Api-Version: 2022-11-28" \
			https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/workflows/$GITHUB_WORKFLOW_ID/runs)
		script=$(printf '.workflow_runs | map(select(.name == "copy %s to %s%s")) | first | .id' "$IMAGE_ID" "$DEST_IMAGE_ID" "$suffix")
		runId=$(echo "$runs" | jq -r "$script")
		if [ "$runId" == "null" ]; then
			sleep 1s
		else
			break
		fi
	done
	link="https://github.com/$GITHUB_OWNER/$GITHUB_REPO/actions/runs/$runId"
	echo "workflow run links: $link"
	# 等待workflow执行结果
	status=""
	while true; do
		run=$(curl -sL \
			-H "Accept: application/vnd.github+json" \
			-H "Authorization: Bearer $GITHUB_TOKEN" \
			-H "X-GitHub-Api-Version: 2022-11-28" \
			https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/runs/$runId)
		status=$(echo "$run" | jq -r '.status')
		if [ "$status" == "completed" ]; then
			conclusion=$(echo "$run" | jq -r '.conclusion')
			echo -e "\rworkflow run $status! conclusion: $conclusion"
			if [ ! "$conclusion" == "success" ]; then
				echo -e "\rworkflow runs failure, see details: $link"
				exit 1
			fi
			break
		else
			echo -ne "\rworkflow run $status..."
			sleep 3s
		fi
	done

	# 拉取镜像
	docker pull "$DEST_IMAGE_ID"
fi
# 本地打tag
docker tag "$DEST_IMAGE_ID" "$IMAGE_ID"
echo "image is sucessfully pulled and tagged as $IMAGE_ID, just use it!"
