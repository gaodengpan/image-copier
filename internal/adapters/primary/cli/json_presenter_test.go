package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPresenter_PresentCheckingImageCount(t *testing.T) {
	p := NewJSONPresenter()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentCheckingImageCount(5)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Equal(t, "", output)
}

func TestJSONPresenter_PresentDiffSummary(t *testing.T) {
	p := NewJSONPresenter()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentDiffSummary(3, 2)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Equal(t, "", output)
}

func TestJSONPresenter_PresentDryRunResults(t *testing.T) {
	p := NewJSONPresenter()

	synced := []syncTask{
		{Source: "nginx:latest", Arch: "amd64", Os: "linux"},
	}
	toSync := []syncTask{
		{Source: "redis:alpine", Arch: "arm64", Os: "linux"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentDryRunResults(synced, toSync)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Equal(t, "", output)
}

func TestJSONPresenter_PresentProgress(t *testing.T) {
	p := NewJSONPresenter()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentProgress(3, 10)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Equal(t, "", output)
}

func TestJSONPresenter_PresentSummary(t *testing.T) {
	p := NewJSONPresenter()

	summary := &PullSummary{
		Succeeded: 2,
		Skipped:   1,
		Failed:    1,
		Duration:  30 * time.Second,
	}
	results := []ImageResult{
		{Image: "nginx:latest", Arch: "amd64", Os: "linux", Success: true},
		{Image: "redis:alpine", Arch: "arm64", Os: "linux", Success: true},
		{Image: "python:3.9", Arch: "amd64", Os: "linux", Skipped: true},
		{Image: "golang:latest", Arch: "amd64", Os: "linux", Failed: true, Error: "network error"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentSummary(summary, results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	summaryObj, ok := result["summary"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(2), summaryObj["Succeeded"])
	assert.Equal(t, float64(1), summaryObj["Skipped"])
	assert.Equal(t, float64(1), summaryObj["Failed"])

	images := result["images"].([]interface{})
	assert.Len(t, images, 4)

	img0 := images[0].(map[string]interface{})
	assert.Equal(t, "nginx:latest", img0["image"])
	assert.Equal(t, "amd64", img0["arch"])
	assert.Equal(t, "linux", img0["os"])
	assert.Equal(t, true, img0["success"])

	img3 := images[3].(map[string]interface{})
	assert.Equal(t, "golang:latest", img3["image"])
	assert.Equal(t, true, img3["failed"])
	assert.Equal(t, "network error", img3["error"])
}

func TestJSONPresenter_PresentSummaryWithDryRun(t *testing.T) {
	p := NewJSONPresenter()

	summary := &PullSummary{
		Succeeded: 0,
		Skipped:   0,
		DryRun:    3,
		Failed:    0,
		Duration:  10 * time.Second,
	}
	results := []ImageResult{
		{Image: "img1", DryRun: true},
		{Image: "img2", DryRun: true},
		{Image: "img3", DryRun: true},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentSummary(summary, results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	summaryObj, ok := result["summary"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(3), summaryObj["DryRun"])

	images := result["images"].([]interface{})
	assert.Len(t, images, 3)
}

func TestJSONPresenter_PresentSummaryWithCancelled(t *testing.T) {
	p := NewJSONPresenter()

	summary := &PullSummary{
		Succeeded: 1,
		Skipped:   0,
		Failed:    1,
	}
	results := []ImageResult{
		{Image: "img1", Success: true},
		{Image: "img2", Cancelled: true, Error: "timeout"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentSummary(summary, results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	images := result["images"].([]interface{})
	img1 := images[1].(map[string]interface{})
	assert.Equal(t, true, img1["cancelled"])
	assert.Equal(t, "timeout", img1["error"])
}

func TestJSONPresenter_PresentError(t *testing.T) {
	p := NewJSONPresenter()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentError(assert.AnError)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var result map[string]string
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, "assert.AnError general error for testing", result["error"])
}

func TestJSONPresenter_PresentSummaryEmpty(t *testing.T) {
	p := NewJSONPresenter()

	summary := &PullSummary{}
	results := []ImageResult{}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.PresentSummary(summary, results)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	images := result["images"].([]interface{})
	assert.Len(t, images, 0)
}
