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
echo "source image $IMAGE_ID,target image $DEST_IMAGE_ID"

# login
docker login -u $REGISTRY_USERNAME -p $REGISTRY_PASSWD $REGISTRY_HOST >/dev/null 2>&1
if [ $? == 1 ]; then
	exit 1
fi
# 拉取目标镜像
docker pull "$DEST_IMAGE_ID" >/dev/null 2>&1
if [ $? == 1 ]; then
	# 触发github workflow同步镜像
	echo "trigger github workflow for image copy"
	data=$(printf '{"ref":"master","inputs":{"imageId":"%s","destImageId":"%s"}}' "$IMAGE_ID" "$DEST_IMAGE_ID")
	curl -sL \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: Bearer $GITHUB_TOKEN" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/workflows/$GITHUB_WORKFLOW_ID/dispatches \
		-d "$data"
	# 等待workflow执行结果
	echo "wait workflow to complete..."
	status=""
	script=$(printf '.workflow_runs | map(select(.name == "copy %s to %s")) | first | .status' "$IMAGE_ID" "$DEST_IMAGE_ID")
	while [ ! "$status" == "completed" ]; do
		runs=$(curl -sL \
			-H "Accept: application/vnd.github+json" \
			-H "Authorization: Bearer $GITHUB_TOKEN" \
			-H "X-GitHub-Api-Version: 2022-11-28" \
			https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/workflows/$GITHUB_WORKFLOW_ID/runs)
		status=$(echo "$runs" | jq "$script" | tr -d '"')
		sleep 5s
	done

	# 拉取镜像
	echo "pull image $DEST_IMAGE_ID"
	docker pull "$DEST_IMAGE_ID"
fi

# 本地打tag
docker tag "$DEST_IMAGE_ID" "$IMAGE_ID"
