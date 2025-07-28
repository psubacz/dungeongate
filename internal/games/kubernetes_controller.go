package games

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/dungeongate/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

// KubernetesController manages game pods in Kubernetes
type KubernetesController struct {
	clientset *kubernetes.Clientset
	namespace string
	config    *config.GameServiceConfig
}

// NewKubernetesController creates a new Kubernetes controller
func NewKubernetesController(namespace string, cfg *config.GameServiceConfig) (*KubernetesController, error) {
	// Create in-cluster config
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &KubernetesController{
		clientset: clientset,
		namespace: namespace,
		config:    cfg,
	}, nil
}

// StartGamePod starts a new game pod for the given session
func (k *KubernetesController) StartGamePod(ctx context.Context, session *GameSession, game *Game) error {
	// Load pod template
	podSpec, err := k.loadPodTemplate(session, game)
	if err != nil {
		return fmt.Errorf("failed to load pod template: %w", err)
	}

	// Create the pod
	pod, err := k.clientset.CoreV1().Pods(k.namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	// Update session with pod information
	session.PodName = pod.Name
	session.ContainerID = fmt.Sprintf("k8s://%s/%s/%s", k.namespace, pod.Name, "nethack")

	// Wait for pod to be ready
	return k.waitForPodReady(ctx, pod.Name, 60*time.Second)
}

// StopGamePod stops and deletes a game pod
func (k *KubernetesController) StopGamePod(ctx context.Context, session *GameSession) error {
	if session.PodName == "" {
		return fmt.Errorf("no pod name found for session %s", session.ID)
	}

	// Gracefully delete the pod
	gracePeriod := int64(30) // 30 seconds grace period
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	return k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, session.PodName, deleteOptions)
}

// GetPodStatus returns the status of a game pod
func (k *KubernetesController) GetPodStatus(ctx context.Context, podName string) (*corev1.PodStatus, error) {
	pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %w", podName, err)
	}

	return &pod.Status, nil
}

// GetPodLogs returns the logs from a game pod
func (k *KubernetesController) GetPodLogs(ctx context.Context, podName string, lines int64) (string, error) {
	podLogOptions := &corev1.PodLogOptions{
		TailLines: &lines,
	}

	request := k.clientset.CoreV1().Pods(k.namespace).GetLogs(podName, podLogOptions)
	logs, err := request.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(logs)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %w", err)
	}

	return buf.String(), nil
}

// WatchGamePods watches for changes in game pods
func (k *KubernetesController) WatchGamePods(ctx context.Context, callback func(*corev1.Pod, watch.EventType)) error {
	watchOptions := metav1.ListOptions{
		LabelSelector: "app=nethack,managed-by=dungeongate-game-service",
		Watch:         true,
	}

	watcher, err := k.clientset.CoreV1().Pods(k.namespace).Watch(ctx, watchOptions)
	if err != nil {
		return fmt.Errorf("failed to create pod watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			callback(pod, event.Type)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ListActivePods returns a list of active game pods
func (k *KubernetesController) ListActivePods(ctx context.Context) ([]*corev1.Pod, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=nethack,managed-by=dungeongate-game-service",
	}

	podList, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var activePods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			activePods = append(activePods, pod)
		}
	}

	return activePods, nil
}

// CleanupFinishedPods removes pods that have finished running
func (k *KubernetesController) CleanupFinishedPods(ctx context.Context, maxAge time.Duration) error {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=nethack,managed-by=dungeongate-game-service",
	}

	podList, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for i := range podList.Items {
		pod := &podList.Items[i]

		// Skip running or pending pods
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			continue
		}

		// Check if pod is old enough to be cleaned up
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			var finishTime time.Time
			if pod.Status.Phase == corev1.PodSucceeded {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
						finishTime = condition.LastTransitionTime.Time
						break
					}
				}
			} else {
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if containerStatus.State.Terminated != nil {
						finishTime = containerStatus.State.Terminated.FinishedAt.Time
						break
					}
				}
			}

			if !finishTime.IsZero() && finishTime.Before(cutoff) {
				err := k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
				if err != nil {
					// Log error but continue with cleanup
					fmt.Printf("Failed to delete finished pod %s: %v\n", pod.Name, err)
				}
			}
		}
	}

	return nil
}

// loadPodTemplate loads and processes the pod template for a game
func (k *KubernetesController) loadPodTemplate(session *GameSession, game *Game) (*corev1.Pod, error) {
	// Get the pod template from ConfigMap
	configMap, err := k.clientset.CoreV1().ConfigMaps(k.namespace).Get(
		context.Background(),
		"nethack-pod-template",
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod template ConfigMap: %w", err)
	}

	templateData, exists := configMap.Data["pod-template.yaml"]
	if !exists {
		return nil, fmt.Errorf("pod-template.yaml not found in ConfigMap")
	}

	// Prepare template variables
	vars := map[string]string{
		"SESSION_ID":        session.ID,
		"USER_ID":           fmt.Sprintf("%d", session.UserID),
		"USERNAME":          session.Username,
		"CREATED_AT":        time.Now().Format(time.RFC3339),
		"RECORDING_ENABLED": "true", // TODO: Make this configurable
	}

	// Process the template
	tmpl, err := template.New("pod-template").Parse(templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pod template: %w", err)
	}

	var processedTemplate bytes.Buffer
	err = tmpl.Execute(&processedTemplate, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to execute pod template: %w", err)
	}

	// Parse the processed YAML
	var pod corev1.Pod
	err = yaml.Unmarshal(processedTemplate.Bytes(), &pod)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pod YAML: %w", err)
	}

	// Apply additional configurations from game config
	k.applyGameConfig(&pod, game)

	return &pod, nil
}

// applyGameConfig applies game-specific configuration to the pod
func (k *KubernetesController) applyGameConfig(pod *corev1.Pod, game *Game) {
	// Find the main container (should be "nethack")
	var container *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "nethack" {
			container = &pod.Spec.Containers[i]
			break
		}
	}

	if container == nil {
		return
	}

	// Apply resource limits from game config
	if game.Resources != nil {
		if container.Resources.Requests == nil {
			container.Resources.Requests = make(corev1.ResourceList)
		}
		if container.Resources.Limits == nil {
			container.Resources.Limits = make(corev1.ResourceList)
		}

		// Set CPU limits
		if game.Resources.CPULimit != "" {
			container.Resources.Limits[corev1.ResourceCPU] = parseResourceQuantity(game.Resources.CPULimit)
		}
		if game.Resources.CPURequest != "" {
			container.Resources.Requests[corev1.ResourceCPU] = parseResourceQuantity(game.Resources.CPURequest)
		}

		// Set memory limits
		if game.Resources.MemoryLimit != "" {
			container.Resources.Limits[corev1.ResourceMemory] = parseResourceQuantity(game.Resources.MemoryLimit)
		}
		if game.Resources.MemoryRequest != "" {
			container.Resources.Requests[corev1.ResourceMemory] = parseResourceQuantity(game.Resources.MemoryRequest)
		}
	}

	// Apply environment variables from game config
	if game.Environment != nil {
		for key, value := range game.Environment {
			// Skip variables that are already set in the template
			found := false
			for _, env := range container.Env {
				if env.Name == key {
					found = true
					break
				}
			}

			if !found {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  key,
					Value: value,
				})
			}
		}
	}

	// Apply container image configuration
	if game.Container != nil {
		if game.Container.Image != "" {
			image := game.Container.Image
			if game.Container.Tag != "" {
				image += ":" + game.Container.Tag
			}
			container.Image = image
		}

		if game.Container.PullPolicy != "" {
			switch strings.ToLower(game.Container.PullPolicy) {
			case "always":
				container.ImagePullPolicy = corev1.PullAlways
			case "never":
				container.ImagePullPolicy = corev1.PullNever
			case "ifnotpresent":
				container.ImagePullPolicy = corev1.PullIfNotPresent
			}
		}
	}
}

// waitForPodReady waits for a pod to be ready or timeout
func (k *KubernetesController) waitForPodReady(ctx context.Context, podName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watchOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
		Watch:         true,
	}

	watcher, err := k.clientset.CoreV1().Pods(k.namespace).Watch(ctx, watchOptions)
	if err != nil {
		return fmt.Errorf("failed to create pod watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Check if pod is ready
			if pod.Status.Phase == corev1.PodRunning {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
						return nil // Pod is ready!
					}
				}
			}

			// Check if pod failed
			if pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod %s failed: %s", podName, pod.Status.Reason)
			}

		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod %s to be ready", podName)
		}
	}
}

// parseResourceQuantity parses a resource quantity string into a Kubernetes resource.Quantity
func parseResourceQuantity(s string) resource.Quantity {
	// Parse the resource quantity string
	quantity, err := resource.ParseQuantity(s)
	if err != nil {
		// Return zero quantity on error
		return resource.Quantity{}
	}
	return quantity
}

// GetPodMetrics returns resource usage metrics for a pod
func (k *KubernetesController) GetPodMetrics(ctx context.Context, podName string) (*PodMetrics, error) {
	// This would typically use the metrics-server API
	// For now, return a placeholder
	return &PodMetrics{
		PodName:     podName,
		CPUUsage:    "0m",
		MemoryUsage: "0Mi",
		Timestamp:   time.Now(),
	}, nil
}

// PodMetrics represents resource usage metrics for a pod
type PodMetrics struct {
	PodName     string    `json:"pod_name"`
	CPUUsage    string    `json:"cpu_usage"`
	MemoryUsage string    `json:"memory_usage"`
	Timestamp   time.Time `json:"timestamp"`
}

// PodEventHandler handles pod events for monitoring and logging
type PodEventHandler struct {
	service *Service
}

// NewPodEventHandler creates a new pod event handler
func NewPodEventHandler(service *Service) *PodEventHandler {
	return &PodEventHandler{
		service: service,
	}
}

// HandlePodEvent processes pod events and updates session state
func (h *PodEventHandler) HandlePodEvent(pod *corev1.Pod, eventType watch.EventType) {
	sessionID, exists := pod.Labels["session-id"]
	if !exists {
		return
	}

	switch eventType {
	case watch.Added:
		h.handlePodAdded(pod, sessionID)
	case watch.Modified:
		h.handlePodModified(pod, sessionID)
	case watch.Deleted:
		h.handlePodDeleted(pod, sessionID)
	}
}

// handlePodAdded handles when a pod is added
func (h *PodEventHandler) handlePodAdded(pod *corev1.Pod, sessionID string) {
	event := &GameEvent{
		EventID:   generateEventID(),
		SessionID: sessionID,
		EventType: "pod_created",
		EventData: []byte(fmt.Sprintf("Pod %s created", pod.Name)),
		Metadata: map[string]string{
			"pod_name":      pod.Name,
			"pod_phase":     string(pod.Status.Phase),
			"pod_uid":       string(pod.UID),
			"creation_time": pod.CreationTimestamp.Format(time.RFC3339),
		},
		Timestamp: time.Now(),
	}

	h.service.PublishGameEvent(event)
}

// handlePodModified handles when a pod is modified
func (h *PodEventHandler) handlePodModified(pod *corev1.Pod, sessionID string) {
	// Update session status based on pod phase
	h.service.sessionMutex.Lock()
	if session, exists := h.service.activeSessions[sessionID]; exists {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			session.IsActive = true
		case corev1.PodSucceeded, corev1.PodFailed:
			session.IsActive = false
		}
	}
	h.service.sessionMutex.Unlock()

	event := &GameEvent{
		EventID:   generateEventID(),
		SessionID: sessionID,
		EventType: "pod_status_changed",
		EventData: []byte(fmt.Sprintf("Pod %s phase: %s", pod.Name, pod.Status.Phase)),
		Metadata: map[string]string{
			"pod_name":  pod.Name,
			"pod_phase": string(pod.Status.Phase),
			"pod_uid":   string(pod.UID),
		},
		Timestamp: time.Now(),
	}

	h.service.PublishGameEvent(event)
}

// handlePodDeleted handles when a pod is deleted
func (h *PodEventHandler) handlePodDeleted(pod *corev1.Pod, sessionID string) {
	// Mark session as inactive
	h.service.sessionMutex.Lock()
	if session, exists := h.service.activeSessions[sessionID]; exists {
		session.IsActive = false
	}
	h.service.sessionMutex.Unlock()

	event := &GameEvent{
		EventID:   generateEventID(),
		SessionID: sessionID,
		EventType: "pod_deleted",
		EventData: []byte(fmt.Sprintf("Pod %s deleted", pod.Name)),
		Metadata: map[string]string{
			"pod_name": pod.Name,
			"pod_uid":  string(pod.UID),
		},
		Timestamp: time.Now(),
	}

	h.service.PublishGameEvent(event)
}

// generateEventID generates a unique event ID
func generateEventID() string {
	// TODO: Implement proper event ID generation
	return fmt.Sprintf("event_%d", time.Now().UnixNano())
}
