package api

import (
	"fmt"
	"net/http"

	"github.com/banzaicloud/pipeline/helm"
	"github.com/banzaicloud/pipeline/internal/platform/gin/utils"
	pkgCommmon "github.com/banzaicloud/pipeline/pkg/common"
	"github.com/banzaicloud/pipeline/pkg/hpa"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"k8s.io/api/autoscaling/v2beta1"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const hpaAnnotationPrefix = "hpa.autoscaling.banzaicloud.io"

// PutHpaResource create/updates a Hpa resource bound to a K8s deployment/statefulset
func PutHpaResource(c *gin.Context) {

	name := c.Param("name")
	kubeConfig, ok := GetK8sConfig(c)

	if !ok {
		log.Errorf("could not get the k8s config for querying the details of deployment: [%s]", name)
		return
	}

	var scalingRequest *hpa.DeploymentScalingRequest
	err := c.BindJSON(&scalingRequest)
	if err != nil {
		err := errors.Wrap(err, "Error parsing request:")
		log.Error(err.Error())
		c.JSON(http.StatusBadRequest, pkgCommmon.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Error during parsing request!",
			Error:   errors.Cause(err).Error(),
		})
	}
	log.Info("Parse deployment succeeded")

	err = SetDeploymentAutoscalingInfo(name, kubeConfig, *scalingRequest)

	c.JSON(http.StatusCreated, "")
}

// GetHpaResource returns a Hpa resource bound to a K8s deployment/statefulset
func GetHpaResource(c *gin.Context) {
	scaleTarget, ok := ginutils.RequiredQuery(c, "scaleTarget")
	if !ok {
		c.JSON(http.StatusBadRequest, pkgCommmon.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "missing required param: scaleTarget",
			Error:   "missing required param: scaleTarget",
		})
		return
	}
	log.Infof("getting hpa details for scaleTarget: [%s]", scaleTarget)

	kubeConfig, ok := GetK8sConfig(c)
	if !ok {
		log.Errorf("could not get the k8s config")
		return
	}

	deploymentResponse, err := GetHpaResources(scaleTarget, kubeConfig)
	if err != nil {
		log.Error("Error during getting deployment details: ", err.Error())

		httpStatusCode := http.StatusInternalServerError
		if _, ok := err.(*helm.DeploymentNotFoundError); ok {
			httpStatusCode = http.StatusBadRequest
		}

		c.JSON(httpStatusCode, pkgCommmon.ErrorResponse{
			Code:    httpStatusCode,
			Message: "Error getting deployment",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, deploymentResponse)

}

func GetHpaResources(scaleTragetRef string, kubeConfig []byte) ([]hpa.DeploymentScalingInfo, error) {
	client, err := helm.GetK8sConnection(kubeConfig)
	if err != nil {
		log.Errorf("Getting K8s client failed: %s", err.Error())
		return nil, err
	}
	responseDeployments := make([]hpa.DeploymentScalingInfo, 0)

	listOption := v12.ListOptions{
		TypeMeta: v12.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		},
	}
	hpaList, err := client.AutoscalingV2beta1().HorizontalPodAutoscalers(v12.NamespaceAll).List(listOption)
	if err != nil {
		log.Errorf("Getting hpa for %v failed: %s", scaleTragetRef, err.Error())
	} else {

		for _, hpaItem := range hpaList.Items {
			if !hpaBelongsToDeployment(hpaItem, scaleTragetRef) {
				continue
			}

			log.Infof("hpa found: %v for scaleTragetRef: %v", hpaItem.Name, scaleTragetRef)
			deploymentItem := hpa.DeploymentScalingInfo{
				Name:        scaleTragetRef,
				Kind:        hpaItem.Spec.ScaleTargetRef.Kind,
				MinReplicas: *hpaItem.Spec.MinReplicas,
				MaxReplicas: hpaItem.Spec.MaxReplicas,
			}

			for _, metric := range hpaItem.Spec.Metrics {
				switch metric.Type {
				case v2beta1.ResourceMetricSourceType:
					switch metric.Resource.Name {
					case v1.ResourceCPU:
						deploymentItem.Cpu = getResourceMetricStatus(hpaItem, metric)
					case v1.ResourceMemory:
						deploymentItem.Memory = getResourceMetricStatus(hpaItem, metric)
					}
				case v2beta1.PodsMetricSourceType:
					//TODO populate custom metric info
					log.Warnf("hpa found: %v with metric type: %v", hpaItem.Name, metric.Type)
				default:
					log.Warnf("hpa found: %v with metric type: %v", hpaItem.Name, metric.Type)
				}
			}

			deploymentItem.Status.Message = generateStatusMessage(hpaItem.Status)

			responseDeployments = append(responseDeployments, deploymentItem)
		}

	}

	return responseDeployments, nil
}
func generateStatusMessage(status v2beta1.HorizontalPodAutoscalerStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type == v2beta1.AbleToScale {
			return fmt.Sprintf("%v=%v : %v", v2beta1.AbleToScale, condition.Status, condition.Message)
		}
	}
	return "n/a"
}

func getResourceMetricStatus(hpaItem v2beta1.HorizontalPodAutoscaler, metric v2beta1.MetricSpec) hpa.ResourceMetricStatus {
	metricStatus := hpa.ResourceMetricStatus{}
	if metric.Resource.TargetAverageUtilization != nil {
		metricStatus.TargetAverageValue = fmt.Sprint(*metric.Resource.TargetAverageUtilization)
		metricStatus.TargetAverageValueType = hpa.PercentageValueType
	} else if metric.Resource.TargetAverageValue != nil {
		metricStatus.TargetAverageValue = metric.Resource.TargetAverageValue.String()
		metricStatus.TargetAverageValueType = hpa.QuantityValueType
	}
	for _, currentMetricStatus := range hpaItem.Status.CurrentMetrics {
		if currentMetricStatus.Resource != nil && currentMetricStatus.Resource.Name == metric.Resource.Name {
			if currentMetricStatus.Resource.CurrentAverageUtilization != nil {
				metricStatus.CurrentAverageValue = fmt.Sprint(*currentMetricStatus.Resource.CurrentAverageUtilization)
				metricStatus.TargetAverageValueType = hpa.PercentageValueType
			} else if !currentMetricStatus.Resource.CurrentAverageValue.IsZero() {
				metricStatus.CurrentAverageValue = currentMetricStatus.Resource.CurrentAverageValue.String()
				metricStatus.CurrentAverageValueType = hpa.QuantityValueType
			}
		}
	}

	return metricStatus
}

func hpaBelongsToDeployment(hpa v2beta1.HorizontalPodAutoscaler, scaleTragetRef string) bool {
	// TODO later may be check gvk as well
	if hpa.Spec.ScaleTargetRef.Name != scaleTragetRef {
		return false
	}
	return true
}

func SetDeploymentAutoscalingInfo(releaseName string, kubeConfig []byte, request hpa.DeploymentScalingRequest) error {
	client, err := helm.GetK8sConnection(kubeConfig)
	if err != nil {
		log.Errorf("Getting K8s client failed: %s", err.Error())
		return err
	}

	// find deployment & update hpa annotations
	// get doesn't work with v12.NamespaceAll only if you specify the namespace exactly
	// deployment, err := client.AppsV1().Deployments(v12.NamespaceAll).Get(request.Name, v12.GetOptions{})

	deploymentList, err := client.AppsV1().Deployments(v12.NamespaceAll).List(v12.ListOptions{})
	for _, dep := range deploymentList.Items {
		if dep.Name == request.Name {
			setupHpaAnnotations(request, dep.Annotations)
			log.Infof("set annotations on deployment: %v", dep.Name)
			_, err = client.AppsV1().Deployments(dep.Namespace).Update(&dep)
		}
	}

	// find statefulset & update hpa annotations
	statefulSetList, err := client.AppsV1().StatefulSets(v12.NamespaceAll).List(v12.ListOptions{})
	for _, stsset := range statefulSetList.Items {
		if stsset.Name == request.Name {
			setupHpaAnnotations(request, stsset.Annotations)
			log.Infof("set annotations on statefulset: %v", stsset.Name)
			_, err = client.AppsV1().StatefulSets(stsset.Namespace).Update(&stsset)
		}
	}

	return nil
}

func setupHpaAnnotations(request hpa.DeploymentScalingRequest, annotations map[string]string) {
	// TODO validation
	annotations[fmt.Sprintf("%v/minReplicas", hpaAnnotationPrefix)] = fmt.Sprint(request.MinReplicas)
	annotations[fmt.Sprintf("%v/maxReplicas", hpaAnnotationPrefix)] = fmt.Sprint(request.MaxReplicas)

	setupResourceMetricAnnotation(annotations, "cpu", request.Cpu)
	setupResourceMetricAnnotation(annotations, "memory", request.Memory)
}

func setupResourceMetricAnnotation(annotations map[string]string, prefix string, resourceMetric hpa.ResourceMetric) {
	if len(resourceMetric.TargetAverageValue) > 0 {
		switch resourceMetric.TargetAverageValueType {
		case hpa.PercentageValueType:
			annotations[fmt.Sprintf("%v.%v/targetAverageUtilization", prefix, hpaAnnotationPrefix)] = resourceMetric.TargetAverageValue
		case hpa.QuantityValueType:
			annotations[fmt.Sprintf("%v.%v/targetAverageValue", prefix, hpaAnnotationPrefix)] = resourceMetric.TargetAverageValue
		}
	}
}
