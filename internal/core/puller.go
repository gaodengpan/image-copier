package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Puller handles the image pulling process
type Puller struct {
	Config *Config
	Logger *logrus.Logger
}

// Config holds the configuration needed for Puller
type Config struct {
	GithubOwner    string
	GithubRepo     string
	GithubToken    string
	GithubWorkflowID string
	RegistryHost   string
	RegistryUsername string
	RegistryPassword string
	RegistryNamespace string
	RegistryArch   string
	RegistryOs     string
}

// NewPuller creates a new Puller instance
func NewPuller(config *Config, logger *logrus.Logger) *Puller {
	return &Puller{
		Config: config,
		Logger: logger,
	}
}

// PullSingle pulls a single image through GitHub Actions
func (p *Puller) PullSingle(ctx context.Context, imageID string) error {
	p.Logger.Infof("Processing image: %s", imageID)

	sourceID := p.normalizeSourceID(imageID)
	destImageID := p.buildDestImageID(sourceID)

	// Login to registry
	if err := p.loginRegistry(); err != nil {
		return fmt.Errorf("failed to login to registry: %w", err)
	}

	// Check if image already exists
	exists, err := p.checkImageExists(destImageID)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	if !exists {
		p.Logger.Info("Image not found in destination registry, triggering GitHub workflow")
		
		// Trigger GitHub workflow
		runID, err := p.triggerWorkflow(sourceID, destImageID)
		if err != nil {
			return fmt.Errorf("failed to trigger workflow: %w", err)
		}

		// Wait for workflow completion
		if err := p.waitForWorkflow(runID); err != nil {
			return fmt.Errorf("workflow failed: %w", err)
		}
	} else {
		p.Logger.Info("Image already exists in destination registry")
	}

	// Copy and import image
	if err := p.copyAndImportImage(destImageID, sourceID); err != nil {
		return fmt.Errorf("failed to copy and import image: %w", err)
	}

	p.Logger.Infof("Successfully processed image: %s", imageID)
	return nil
}

func (p *Puller) normalizeSourceID(imageID string) string {
	segs := strings.Split(imageID, "/")
	
	switch len(segs) {
	case 1:
		// No registry specified, assume docker.io/library
		return fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		// Check if first segment looks like a domain
		if !strings.Contains(segs[0], ".") && !strings.Contains(segs[0], ":") {
			// Not a domain, prepend docker.io
			return fmt.Sprintf("docker.io/%s", imageID)
		}
		return imageID
	default:
		// Already fully qualified
		return imageID
	}
}

func (p *Puller) buildDestImageID(sourceID string) string {
	if p.Config.RegistryNamespace == "" {
		return fmt.Sprintf("%s/%s", p.Config.RegistryHost, sourceID)
	}
	
	// Replace slashes with underscores and limit length
	normalized := strings.ReplaceAll(sourceID, "/", "_")
	if len(normalized) > 40 {
		normalized = normalized[:40]
	}
	
	return fmt.Sprintf("%s/%s/%s", p.Config.RegistryHost, p.Config.RegistryNamespace, normalized)
}

func (p *Puller) loginRegistry() error {
	cmd := exec.Command("docker", "login", "-u", p.Config.RegistryUsername, "--password-stdin", p.Config.RegistryHost)
	cmd.Stdin = strings.NewReader(p.Config.RegistryPassword)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login failed: %s, output: %s", err, string(output))
	}
	
	return nil
}

func (p *Puller) checkImageExists(destImageID string) (bool, error) {
	creds := fmt.Sprintf("%s:%s", p.Config.RegistryUsername, p.Config.RegistryPassword)
	cmd := exec.Command("skopeo", "inspect", "--creds="+creds, "docker://"+destImageID)
	
	_, err := cmd.Output()
	if err != nil {
		// If command fails, assume image doesn't exist
		return false, nil
	}
	
	return true, nil
}

func (p *Puller) triggerWorkflow(sourceID, destImageID string) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())
	
	data := map[string]interface{}{
		"ref": "master",
		"inputs": map[string]string{
			"imageId":      sourceID,
			"destImageId":  destImageID,
			"suffix":       suffix,
			"arch":         p.Config.RegistryArch,
			"os":           p.Config.RegistryOs,
		},
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}
	
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches", 
		p.Config.GithubOwner, p.Config.GithubRepo, p.Config.GithubWorkflowID)
	
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Find the workflow run ID
	runID, err := p.findWorkflowRunID(sourceID, destImageID, suffix)
	if err != nil {
		return "", fmt.Errorf("failed to find workflow run ID: %w", err)
	}
	
	p.Logger.Infof("Triggered workflow run ID: %s", runID)
	return runID, nil
}

func (p *Puller) findWorkflowRunID(sourceID, destImageID, suffix string) (string, error) {
	expectedName := fmt.Sprintf("copy %s to %s%s", sourceID, destImageID, suffix)
	
	for i := 0; i < 30; i++ { // Retry for up to 30 seconds
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs", 
			p.Config.GithubOwner, p.Config.GithubRepo, p.Config.GithubWorkflowID)
		
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to send request: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		var result struct {
			WorkflowRuns []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"workflow_runs"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()
		
		for _, run := range result.WorkflowRuns {
			if run.Name == expectedName {
				return fmt.Sprintf("%d", run.ID), nil
			}
		}
		
		time.Sleep(time.Second)
	}
	
	return "", fmt.Errorf("workflow run not found after 30 attempts")
}

func (p *Puller) waitForWorkflow(runID string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s", 
		p.Config.GithubOwner, p.Config.GithubRepo, runID)
	
	link := fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s", 
		p.Config.GithubOwner, p.Config.GithubRepo, runID)
	
	p.Logger.Infof("Workflow run link: %s", link)
	
	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		var run struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()
		
		switch run.Status {
		case "completed":
			if run.Conclusion == "success" {
				p.Logger.Info("Workflow completed successfully")
				return nil
			}
			return fmt.Errorf("workflow failed with conclusion: %s", run.Conclusion)
		case "queued", "in_progress":
			p.Logger.Infof("Workflow is %s...", run.Status)
			time.Sleep(3 * time.Second)
		default:
			p.Logger.Warnf("Unknown workflow status: %s", run.Status)
			time.Sleep(3 * time.Second)
		}
	}
}

func (p *Puller) copyAndImportImage(destImageID, sourceID string) error {
	tmpFile := fmt.Sprintf("tmp-%d.tar", rand.Int())
	defer os.Remove(tmpFile)
	
	creds := fmt.Sprintf("%s:%s", p.Config.RegistryUsername, p.Config.RegistryPassword)
	cmd := exec.Command("skopeo", "copy", "--src-creds="+creds, 
		"docker://"+destImageID, "docker-archive:"+tmpFile+":"+sourceID)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy failed: %s, output: %s", err, string(output))
	}
	
	cmd = exec.Command("docker", "load", "-i", tmpFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker load failed: %s, output: %s", err, string(output))
	}
	
	return nil
}
