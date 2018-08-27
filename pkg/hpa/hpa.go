package hpa

type ValueType string

var (
	// PercentageValueType
	PercentageValueType ValueType = "percentage"
	// QuantityValueType
	QuantityValueType ValueType = "quantity"
)

type ResourceMetric struct {
	TargetAverageValueType ValueType `json:"targetAverageValueType,omitempty"`
	TargetAverageValue     string `json:"targetAverageValue,omitempty"`
}

type ResourceMetricStatus struct {
	ResourceMetric
	CurrentAverageValueType ValueType `json:"currentAverageUtilization,omitempty"`
	CurrentAverageValue     string `json:"currentAverageValue,omitempty"`
}

type CustomMetric struct {
	Type               string `json:"type"`
	TargetAverageValue string `json:"targetAverageValue,omitempty"`
}

type CustomMetricStatus struct {
	CustomMetric
	CurrentAverageValue string `json:"currentAverageValue,omitempty"`
}

type DeploymentScaleStatus struct {
	CurrentReplicas int32  `json:"currentReplicas,omitempty"`
	DesiredReplicas int32  `json:"desiredReplicas,omitempty"`
	Message         string `json:"message,omitempty"`
}

type DeploymentScalingRequest struct {
	Name          string         `json:"name,omitempty"`
	MinReplicas   int32          `json:"minReplicas,omitempty"`
	MaxReplicas   int32          `json:"maxReplicas,omitempty"`
	Cpu           ResourceMetric `json:"cpu,omitempty"`
	Memory        ResourceMetric `json:"memory,omitempty"`
	CustomMetrics []CustomMetric `json:"customMetrics,omitempty"`
}

type DeploymentScalingInfo struct {
	Name          string                `json:"name,omitempty"`
	Kind          string                `json:"kind,omitempty"`
	MinReplicas   int32                 `json:"minReplicas,omitempty"`
	MaxReplicas   int32                 `json:"maxReplicas,omitempty"`
	Cpu           ResourceMetricStatus  `json:"cpu,omitempty"`
	Memory        ResourceMetricStatus  `json:"memory,omitempty"`
	CustomMetrics map[string]CustomMetricStatus  `json:"customMetrics,omitempty"`
	Status        DeploymentScaleStatus `json:"status,omitempty"`
}
