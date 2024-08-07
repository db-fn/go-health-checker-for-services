package main

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

func CheckServiceStatus(serviceName string) string {
	cmd := exec.Command("rc-service", serviceName, "status")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Error checking service status for %s: %v", serviceName, err)
		return "error"
	}
	status := strings.TrimSpace(out.String())
	log.Printf("Service %s, Status: %s", serviceName, status)
	if strings.Contains(status, "started") {
		return "ok"
	}
	return "error"
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
