#! /bin/bash
# set -x
IMAGE_ID=$1
SOURCE_ID=$IMAGE_ID
# 源镜像ID
segs=$(echo "$IMAGE_ID" | awk -F'/' '{print NF}')
if [ ! "$segs" == "3" ]; then
	if [[ "$segs" == "1" ]]; then
		SOURCE_ID=library/$IMAGE_ID
		IMAGE_ID=docker.io/library/$IMAGE_ID
	else
		seg1=$(echo "$IMAGE_ID" | cut -d'/' -f1)
		if [[ ! "$seg1" =~ ^.*(\..*){1,} ]]; then
			IMAGE_ID=docker.io/$IMAGE_ID
		fi
	fi
fi
echo $SOURCE_ID
# 构建目标镜像ID
DEST_IMAGE_ID=""
if [ -z "$REGISTRY_NAMESPACE" ]; then
	DEST_IMAGE_ID=$REGISTRY_HOST/$IMAGE_ID
else
	DEST_IMAGE_ID=$REGISTRY_HOST/$REGISTRY_NAMESPACE/$(echo "$IMAGE_ID" | tr '/' '_' | cut -c 1-40)
fi
echo $DEST_IMAGE_ID

# login
echo "$REGISTRY_PASSWD" | docker login -u "$REGISTRY_USERNAME" --password-stdin "$REGISTRY_HOST"
if [ $? == 1 ]; then
	exit 1
fi
# 检查镜像是否存在
skopeo inspect --creds="$REGISTRY_USERNAME:$REGISTRY_PASSWD" docker://$DEST_IMAGE_ID >/dev/null 2>&1
if [ $? != 0 ]; then
	echo "image not found: $DEST_IMAGE_ID"
	# 触发github workflow同步镜像
	echo "dispatch github workflow: image-copier"
	suffix="--$(date '+%s')"
	data=$(printf '{"ref":"master","inputs":{"imageId":"%s","destImageId":"%s","suffix":"%s","arch":"%s","os":"%s"}}' "$IMAGE_ID" "$DEST_IMAGE_ID" "$suffix" "$REGISTRY_ARCH" "$REGISTRY_OS")
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
			sleep 1
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
			sleep 3
		fi
	done
fi

# 拷贝并导入镜像
skopeo copy --src-creds="$REGISTRY_USERNAME:$REGISTRY_PASSWD" docker://$DEST_IMAGE_ID docker-archive:tmp.tar && docker load -i tmp.tar && rm -rf tmp.tar
# 刷新镜像元数据
docker pull $IMAGE_ID

# 推送到本地仓库
# if [ ! -z "$LOCAL_REGISTRY" ];then
	# docker tag "$IMAGE_ID" $LOCAL_REGISTRY/"$SOURCE_ID"
	# docker push $LOCAL_REGISTRY/"$SOURCE_ID"
# fi
