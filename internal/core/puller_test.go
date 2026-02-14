package core

import (
	"testing"
)

func TestBuildDestImageID(t *testing.T) {
	tests := []struct {
		name             string
		registryHost     string
		registryNamespace string
		sourceID         string
		expected         string
	}{
		{
			name:             "empty host and namespace",
			registryHost:     "",
			registryNamespace: "",
			sourceID:         "nginx:latest",
			expected:         "/nginx_latest",
		},
		{
			name:             "empty host with namespace",
			registryHost:     "",
			registryNamespace: "copies",
			sourceID:         "nginx:latest",
			expected:         "/copies/nginx_latest",
		},
		{
			name:             "host with empty namespace",
			registryHost:     "registry.example.com",
			registryNamespace: "",
			sourceID:         "ghcr.io/tektoncd-catalog/git-clone:v1.1",
			expected:         "registry.example.com/ghcr_io_tektoncd_catalog_git_clone_v1_1",
		},
		{
			name:             "host with namespace",
			registryHost:     "registry.example.com",
			registryNamespace: "copies",
			sourceID:         "ghcr.io/tektoncd-catalog/git-clone:v1.1",
			expected:         "registry.example.com/copies/ghcr_io_tektoncd_catalog_git_clone_v1_1",
		},
		{
			name:             "host with namespace - original problem case",
			registryHost:     "registry.cn-hangzhou.aliyuncs.com",
			registryNamespace: "copies0",
			sourceID:         "ghcr.io/tektoncd-catalog/git-clone:v1.1",
			expected:         "registry.cn-hangzhou.aliyuncs.com/copies0/ghcr_io_tektoncd_catalog_git_clone_v1_1",
		},
		{
			name:             "complex image name with multiple dots and hyphens",
			registryHost:     "registry.example.com",
			registryNamespace: "test",
			sourceID:         "my.company.com/app:2.3.4-beta",
			expected:         "registry.example.com/test/my_company_com_app_2_3_4_beta",
		},
		{
			name:             "single dot in tag",
			registryHost:     "registry.example.com",
			registryNamespace: "copies",
			sourceID:         "library/ubuntu:20.04",
			expected:         "registry.example.com/copies/library_ubuntu_20_04",
		},
		{
			name:             "hyphens in various positions",
			registryHost:     "registry.example.com",
			registryNamespace: "ns",
			sourceID:         "my-app/service-image:latest-beta",
			expected:         "registry.example.com/ns/my_app_service_image_latest_beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildDestImageID(tt.registryHost, tt.registryNamespace, tt.sourceID)
			if result != tt.expected {
				t.Errorf("BuildDestImageID(%q, %q, %q) = %q; expected %q",
					tt.registryHost, tt.registryNamespace, tt.sourceID, result, tt.expected)
			}
		})
	}
}