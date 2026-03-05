package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func CheckServiceStatus(serviceName string) string {
	cmd := exec.Command("nsenter", "--target", "1", "--mount", "--", "systemctl", "is-active", serviceName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	status := strings.TrimSpace(out.String())
	log.Printf("Service %s, Status: %s", serviceName, status)
	if err != nil && status != "active" {
		return "error"
	}
	if status == "active" {
		return "ok"
	}
	return "error"
}

func GetMemoryInfo() map[string]string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	defer f.Close()

	values := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = val
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	used := total - available

	toMB := func(kb uint64) string {
		return fmt.Sprintf("%dMB", kb/1024)
	}

	usedPct := uint64(0)
	if total > 0 {
		usedPct = used * 100 / total
	}

	return map[string]string{
		"total":     toMB(total),
		"used":      toMB(used),
		"available": toMB(available),
		"used_pct":  fmt.Sprintf("%d%%", usedPct),
	}
}

func GetLoadAverage() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "error"
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return string(data)
	}
	return fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2])
}

type RegistryInfo struct {
	TotalRepos int            `json:"total_repos"`
	TotalTags  int            `json:"total_tags"`
	Repos      map[string]int `json:"repos"`
}

func registryGet(url, user, pass string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	return http.DefaultClient.Do(req)
}

func GetRegistryInfo(registryURL, user, pass string) RegistryInfo {
	info := RegistryInfo{Repos: map[string]int{}}

	resp, err := registryGet(registryURL+"/v2/_catalog", user, pass)
	if err != nil {
		log.Printf("Registry catalog error: %v", err)
		return info
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		log.Printf("Registry catalog parse error: %v", err)
		return info
	}

	info.TotalRepos = len(catalog.Repositories)

	for _, repo := range catalog.Repositories {
		tagsResp, err := registryGet(fmt.Sprintf("%s/v2/%s/tags/list", registryURL, repo), user, pass)
		if err != nil {
			continue
		}
		tagsBody, _ := io.ReadAll(tagsResp.Body)
		tagsResp.Body.Close()

		var tagsList struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(tagsBody, &tagsList); err != nil {
			continue
		}
		info.Repos[repo] = len(tagsList.Tags)
		info.TotalTags += len(tagsList.Tags)
	}

	return info
}

func CheckDockerContainerStatus(containerName string) string {
	cmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--filter", "status=running", "--format", "{{.Names}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Error checking container status for %s: %v", containerName, err)
		return "error"
	}
	output := strings.TrimSpace(out.String())
	log.Printf("Container %s, Status: %s", containerName, output)
	if strings.Contains(output, containerName) {
		return "ok"
	}
	return "error"
}

func CheckDiskSpace() string {
	cmd := exec.Command("df", "-h")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Error checking disk space: %v", err)
		return "error"
	}
	return strings.Split(out.String(), "\n")[1]
}

func CountContainersByFilter(nameFilter string) int {
	if nameFilter == "" {
		return 0
	}
	cmd := exec.Command("docker", "ps", "--filter", "name="+nameFilter, "--format", "{{.Names}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func splitEnv(key string) []string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(val, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func main() {
	registryURL := os.Getenv("REGISTRY_URL")
	registryUser := os.Getenv("REGISTRY_USER")
	registryPass := os.Getenv("REGISTRY_PASS")
	if registryURL == "" {
		registryURL = "http://registry:5000"
	}

	previewFilter := os.Getenv("PREVIEW_FILTER")
	previewContainersPerEnv := 5
	if v := os.Getenv("PREVIEW_CONTAINERS_PER_ENV"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			previewContainersPerEnv = n
		}
	}

	defaultServices := []string{
		"docker",
		"github-actions-runner",
		"github-actions-runner-2",
		"github-actions-runner-3",
		"github-actions-runner-4",
		"gitlab-runner",
	}
	defaultContainers := []string{
		"registry",
		"metabase",
	}

	r := gin.Default()

	r.GET("healthcheck", func(c *gin.Context) {
		services := splitEnv("SERVICES")
		if services == nil {
			services = defaultServices
		}
		containers := splitEnv("CONTAINERS")
		if containers == nil {
			containers = defaultContainers
		}
		totalPreviewContainers := CountContainersByFilter(previewFilter)
		previewEnvCount := 0
		if previewFilter != "" && totalPreviewContainers > 0 {
			previewEnvCount = totalPreviewContainers / previewContainersPerEnv
		}

		status := map[string]interface{}{
			"services":      map[string]string{},
			"containers":    map[string]string{},
			"diskspace":     CheckDiskSpace(),
			"memory":        GetMemoryInfo(),
			"load":          GetLoadAverage(),
			"registry":      GetRegistryInfo(registryURL, registryUser, registryPass),
			"preview_count": previewEnvCount,
		}

		for _, service := range services {
			status["services"].(map[string]string)[service] = CheckServiceStatus(service)
		}

		for _, container := range containers {
			status["containers"].(map[string]string)[container] = CheckDockerContainerStatus(container)
		}

		c.JSON(http.StatusOK, status)
	})

	r.Run(":8000")
}
