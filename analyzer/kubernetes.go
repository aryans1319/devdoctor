package analyzer

import (
	"os"
	"strings"

	"github.com/aryans1319/devdoctor/models"
	"gopkg.in/yaml.v3"
)

// k8s resource structs — covers Deployment, DaemonSet, StatefulSet
type k8sResource struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   k8sMetadata `yaml:"metadata"`
	Spec       k8sSpec     `yaml:"spec"`
}

type k8sMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type k8sSpec struct {
	Template k8sPodTemplate `yaml:"template"`
}

type k8sPodTemplate struct {
	Spec k8sPodSpec `yaml:"spec"`
}

type k8sPodSpec struct {
	Containers         []k8sContainer `yaml:"containers"`
	InitContainers     []k8sContainer `yaml:"initContainers"`
	SecurityContext    *k8sPodSecurityContext `yaml:"securityContext"`
}

type k8sContainer struct {
	Name            string               `yaml:"name"`
	Image           string               `yaml:"image"`
	Resources       *k8sResources        `yaml:"resources"`
	LivenessProbe   *k8sProbe            `yaml:"livenessProbe"`
	ReadinessProbe  *k8sProbe            `yaml:"readinessProbe"`
	SecurityContext *k8sSecurityContext  `yaml:"securityContext"`
}

type k8sResources struct {
	Limits   map[string]string `yaml:"limits"`
	Requests map[string]string `yaml:"requests"`
}

type k8sProbe struct {
	HTTPGet   *k8sHTTPGet `yaml:"httpGet"`
	Exec      *k8sExec    `yaml:"exec"`
	TCPSocket interface{} `yaml:"tcpSocket"`
}

type k8sHTTPGet struct {
	Path string      `yaml:"path"`
	Port interface{} `yaml:"port"`
}

type k8sExec struct {
	Command []string `yaml:"command"`
}

type k8sSecurityContext struct {
	RunAsNonRoot           *bool `yaml:"runAsNonRoot"`
	RunAsUser              *int  `yaml:"runAsUser"`
	AllowPrivilegeEscalation *bool `yaml:"allowPrivilegeEscalation"`
	Privileged             *bool `yaml:"privileged"`
	ReadOnlyRootFilesystem *bool `yaml:"readOnlyRootFilesystem"`
}

type k8sPodSecurityContext struct {
	RunAsNonRoot *bool `yaml:"runAsNonRoot"`
	RunAsUser    *int  `yaml:"runAsUser"`
}

func analyzeKubernetes(path string) models.FileResult {
	result := models.FileResult{
		FilePath: path,
		FileType: "kubernetes",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}

	result.Issues = checkKubernetesRules(data, path)
	result.Score = calculateScore(len(result.Issues))
	return result
}

func checkKubernetesRules(data []byte, filePath string) []models.Issue {
	var issues []models.Issue

	// Parse as generic map first to detect kind
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityError,
			Rule:     "INVALID_YAML",
			Message:  "Kubernetes manifest is not valid YAML",
		})
		return issues
	}

	// Only analyze workload kinds that have pod specs
	kind, _ := raw["kind"].(string)
	workloadKinds := map[string]bool{
		"Deployment": true, "DaemonSet": true,
		"StatefulSet": true, "Job": true, "CronJob": true,
	}
	if !workloadKinds[kind] {
		return issues
	}

	var resource k8sResource
	if err := yaml.Unmarshal(data, &resource); err != nil {
		return issues
	}

	// Check namespace
	if resource.Metadata.Namespace == "" {
		issues = append(issues, models.Issue{
			Severity: models.SeverityWarning,
			Rule:     "NO_NAMESPACE",
			Message:  "Resource '" + resource.Metadata.Name + "' has no namespace — use explicit namespaces for multi-tenant clusters",
		})
	}

	containers := resource.Spec.Template.Spec.Containers
	containers = append(containers, resource.Spec.Template.Spec.InitContainers...)

	for _, c := range containers {
		cIssues := checkContainerRules(c, resource.Metadata.Name)
		issues = append(issues, cIssues...)
	}

	return issues
}

func checkContainerRules(c k8sContainer, resourceName string) []models.Issue {
	var issues []models.Issue
	ref := "container '" + c.Name + "' in '" + resourceName + "'"

	// Latest tag
	if strings.HasSuffix(c.Image, ":latest") || !strings.Contains(c.Image, ":") {
		issues = append(issues, models.Issue{
			Severity: models.SeverityError,
			Rule:     "K8S_LATEST_IMAGE_TAG",
			Message:  ref + " uses unpinned image '" + c.Image + "' — pin to a specific digest or version tag",
		})
	}

	// Missing resource limits
	if c.Resources == nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityError,
			Rule:     "K8S_NO_RESOURCE_LIMITS",
			Message:  ref + " has no resource limits — container can consume unbounded CPU/memory and starve other pods",
		})
	} else {
		if c.Resources.Limits == nil {
			issues = append(issues, models.Issue{
				Severity: models.SeverityError,
				Rule:     "K8S_NO_RESOURCE_LIMITS",
				Message:  ref + " has no resource limits defined",
			})
		}
		if c.Resources.Requests == nil {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "K8S_NO_RESOURCE_REQUESTS",
				Message:  ref + " has no resource requests — scheduler cannot make optimal placement decisions",
			})
		}
	}

	// Missing liveness probe
	if c.LivenessProbe == nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityWarning,
			Rule:     "K8S_NO_LIVENESS_PROBE",
			Message:  ref + " has no liveness probe — Kubernetes cannot detect and restart deadlocked containers",
		})
	}

	// Missing readiness probe
	if c.ReadinessProbe == nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityWarning,
			Rule:     "K8S_NO_READINESS_PROBE",
			Message:  ref + " has no readiness probe — traffic may be routed to containers that are not yet ready",
		})
	}

	// Security context checks
	if c.SecurityContext == nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityWarning,
			Rule:     "K8S_NO_SECURITY_CONTEXT",
			Message:  ref + " has no securityContext — container may run as root with elevated privileges",
		})
	} else {
		sc := c.SecurityContext
		if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "K8S_RUNS_AS_ROOT",
				Message:  ref + " does not set runAsNonRoot: true — container may run as root",
			})
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "K8S_PRIVILEGE_ESCALATION",
				Message:  ref + " does not set allowPrivilegeEscalation: false — process can gain more privileges than its parent",
			})
		}
		if sc.Privileged != nil && *sc.Privileged {
			issues = append(issues, models.Issue{
				Severity: models.SeverityError,
				Rule:     "K8S_PRIVILEGED_CONTAINER",
				Message:  ref + " runs as privileged — has full access to host system, major security risk",
			})
		}
	}

	return issues
}

// AnalyzeKubernetesContent analyzes a Kubernetes YAML from raw string content
// Used by the GitHub App which fetches file content from GitHub API
func AnalyzeKubernetesContent(content, filePath string) models.FileResult {
	result := models.FileResult{
		FilePath: filePath,
		FileType: "kubernetes",
	}

	result.Issues = checkKubernetesRules([]byte(content), filePath)
	result.Score = calculateScore(len(result.Issues))
	return result
}

func IsKubernetesFile(filename string) bool {
	lower := strings.ToLower(filename)
	// Must be a YAML file
	if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
		return false
	}
	// Skip docker-compose files
	if strings.Contains(lower, "docker-compose") || strings.Contains(lower, "docker_compose") {
		return false
	}
	// Known k8s paths
	k8sPaths := []string{"k8s/", "kubernetes/", "kube/", "manifests/", "deploy/", "deployments/", "helm/"}
	for _, p := range k8sPaths {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// Known k8s filename patterns
	k8sPatterns := []string{
		"deployment", "service", "ingress", "configmap", "secret",
		"statefulset", "daemonset", "cronjob", "job", "namespace",
		"persistentvolume", "hpa", "pod",
	}
	base := filename
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		base = filename[idx+1:]
	}
	baseLower := strings.ToLower(base)
	for _, p := range k8sPatterns {
		if strings.Contains(baseLower, p) {
			return true
		}
	}
	return false
}