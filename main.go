package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func CheckServiceStatus(serviceName string) string {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		status := strings.TrimSpace(out.String())
		log.Printf("Service %s (systemctl), Status: %s", serviceName, status)
		if status == "active" {
			return "ok"
		}
		return "error"
	}

	out.Reset()
	cmd = exec.Command("rc-service", serviceName, "status")
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Printf("Error checking service status for %s: %v", serviceName, err)
		return "error"
	}
	status := strings.TrimSpace(out.String())
	log.Printf("Service %s (rc-service), Status: %s", serviceName, status)
	if strings.Contains(status, "started") {
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

func main() {
	r := gin.Default()

	r.GET("healthcheck", func(c *gin.Context) {
		services := []string{
			"docker",
			"github-actions-runner",
			"github-actions-runner-2",
			"github-actions-runner-3",
			"github-actions-runner-4",
			"gitlab-runner",
		}
		containers := []string{
			"ce0c0dad49d2481ea4b9bde4e7c879b4_postgres128alpine_9414f5",
			"7466052210e349bb99c2997bf09ba5da_python3914_0e925c",
			"outline-docker-compose-wk-nginx-1",
			"outline-docker-compose-wk-outline-1",
			"outline-docker-compose-wk-minio-1",
			"outline-docker-compose-wk-oidc-server-1",
			"outline-docker-compose-wk-redis-1",
			"outline-docker-compose-wk-postgres-1",
			"metabase",
		}
		status := map[string]interface{}{
			"services":   map[string]string{},
			"containers": map[string]string{},
			"diskspace":  CheckDiskSpace(),
			"memory":     GetMemoryInfo(),
			"load":       GetLoadAverage(),
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
