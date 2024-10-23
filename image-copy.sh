#! /bin/bash

# TODO 脱敏
REGISTRY_HOST="registry.cn-hangzhou.aliyuncs.com"
REGISTRY_NAMESPACE="copies"
REGISTRY_USERNAME="gaodp"
REGISTRY_PASSWD="wgll0812"
GITHUB_TOKEN="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"
GITHUB_WORKFLOW_ID="123569246"
GITHUB_OWNER="gaodengpan"
GITHUB_REPO="image-copier"

IMAGE_ID=$1
# 构建目标镜像ID
DEST_IMAGE_ID=""
if [ -z "$REGISTRY_NAMESPACE" ]; then
	DEST_IMAGE_ID=$REGISTRY_HOST/$IMAGE_ID
else
	DEST_IMAGE_ID=$REGISTRY_HOST/$REGISTRY_NAMESPACE/$(echo "$IMAGE_ID" | tr '/' '-')
fi

# login
echo "login $REGISTRY_HOST"
docker login -u $REGISTRY_USERNAME -p $REGISTRY_PASSWD $REGISTRY_HOST >/dev/null 2>&1
if [ $? == 1 ]; then
	echo "login $REGISTRY_HOST failed"
	exit 1
fi
# 尝试拉取目标镜像
docker pull "$DEST_IMAGE_ID" >/dev/null 2>&1
if [ $? == 1 ]; then
	echo "unable find $DEST_IMAGE_ID"
	# 触发github workflow同步镜像
	echo "dispatch github workflow for image copy"
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
	runs=$(curl -sL \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: Bearer $GITHUB_TOKEN" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/workflows/$GITHUB_WORKFLOW_ID/runs)
	script=$(printf '.workflow_runs | map(select(.name == "copy %s to %s%s")) | first | .id' "$IMAGE_ID" "$DEST_IMAGE_ID" "$suffix")
	runId=$(echo "$runs" | jq -r "$script")
	echo "runId $runId"
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
			echo "workflow $status"
			break
		else
			echo -ne "\rworkflow $status..."
			sleep 3s
		fi
	done

	# 拉取镜像
	echo "pull image $DEST_IMAGE_ID"
	docker pull "$DEST_IMAGE_ID"
fi

# 本地打tag
docker tag "$DEST_IMAGE_ID" "$IMAGE_ID"
