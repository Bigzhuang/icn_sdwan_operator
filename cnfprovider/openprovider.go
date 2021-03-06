package cnfprovider

import (
	"context"
	"errors"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"reflect"
	"sdewan.akraino.org/sdewan/controllers"
	"sdewan.akraino.org/sdewan/openwrt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("OpenWrtProvider")

type OpenWrtProvider struct {
	Namespace     string
	SdewanPurpose string
	Deployment    extensionsv1beta1.Deployment
	K8sClient     client.Client
}

func NewOpenWrt(namespace string, sdewanPurpose string, k8sClient client.Client) (*OpenWrtProvider, error) {
	reqLogger := log.WithValues("namespace", namespace, "sdewanPurpose", sdewanPurpose)
	ctx := context.Background()
	deployments := &extensionsv1beta1.DeploymentList{}
	err := k8sClient.List(ctx, deployments, client.MatchingLabels{"sdewanPurpose": sdewanPurpose})
	if err != nil {
		reqLogger.Error(err, "Failed to get cnf deployment")
		return nil, client.IgnoreNotFound(err)
	}
	if len(deployments.Items) != 1 {
		reqLogger.Error(nil, "More than one deployment exists")
		return nil, errors.New("More than one deployment exists")
	}

	return &OpenWrtProvider{namespace, sdewanPurpose, deployments.Items[0], k8sClient}, nil
}

func (p *OpenWrtProvider) AddOrUpdateObject(handler ISdewanHandler, instance runtime.Object) (bool, error) {
	// reqLogger := log.WithValues("Mwan3Policy", mwan3Policy.Name, "cnf", p.Deployment.Name)
	reqLogger := log.WithValues(handler.GetType(), handler.GetName(instance), "cnf", p.Deployment.Name)
	ctx := context.Background()
	podList := &corev1.PodList{}
	err := p.K8sClient.List(ctx, podList, client.MatchingLabels{"sdewanPurpose": p.SdewanPurpose})
	if err != nil {
		reqLogger.Error(err, "Failed to get cnf pod list")
		return false, err
	}
	// policy, err := p.convertCrd(mwan3Policy)
	new_instance, err := handler.Convert(instance)
	if err != nil {
		reqLogger.Error(err, "Failed to convert CR for " + handler.GetType())
		return false, err
	}
	cnfChanged := false
	for _, pod := range podList.Items {
		// openwrtClient := openwrt.GetOpenwrtClient(pod.Status.PodIP, "root", "")
		// mwan3 := openwrt.Mwan3Client{OpenwrtClient: openwrtClient}
		// service := openwrt.ServiceClient{OpenwrtClient: openwrtClient}
		// runtimePolicy, _ := mwan3.GetPolicy(policy.Name)
		clientInfo := &openwrt.OpenwrtClientInfo{pod.Status.PodIP, "root", ""}
		runtime_instance, _ := handler.GetObject(clientInfo, new_instance.GetName())
		changed := false
		// if runtimePolicy == nil {
		if runtime_instance == nil {
			// _, err := mwan3.CreatePolicy(*policy)
			_, err := handler.CreateObject(clientInfo, new_instance)
			if err != nil {
				reqLogger.Error(err, "Failed to create " + handler.GetType())
				return false, err
			}
			changed = true
		// } else if reflect.DeepEqual(*runtimePolicy, *policy) {
		} else if handler.IsEqual(runtime_instance, new_instance) {
			reqLogger.Info("Equal to the runtime instance, so no update")
		} else {
			// _, err := mwan3.UpdatePolicy(*policy)
			_, err := handler.UpdateObject(clientInfo, new_instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update " + handler.GetType())
				return false, err
			}
			changed = true
		}
		if changed {
			// _, err = service.ExecuteService("mwan3", "restart")
			_, err = handler.Restart(clientInfo)
			if err != nil {
				reqLogger.Error(err, "Failed to restart openwrt service")
				return changed, err
			}
			cnfChanged = true
		}
	}
	// We say the AddUpdate succeed only when the add/update for all pods succeed
	return cnfChanged, nil
}

func (p *OpenWrtProvider) DeleteObject(handler ISdewanHandler, instance runtime.Object) (bool, error) {
	// reqLogger := log.WithValues("Mwan3Policy", mwan3Policy.Name, "cnf", p.Deployment.Name)
	reqLogger := log.WithValues(handler.GetType(), handler.GetName(instance), "cnf", p.Deployment.Name)
	ctx := context.Background()
	podList := &corev1.PodList{}
	err := p.K8sClient.List(ctx, podList, client.MatchingLabels{"sdewanPurpose": p.SdewanPurpose})
	if err != nil {
		reqLogger.Error(err, "Failed to get pod list")
		return false, err
	}
	cnfChanged := false
	for _, pod := range podList.Items {
		// openwrtClient := openwrt.NewOpenwrtClient(pod.Status.PodIP, "root", "")
		// mwan3 := openwrt.Mwan3Client{OpenwrtClient: openwrtClient}
		// service := openwrt.ServiceClient{OpenwrtClient: openwrtClient}
		clientInfo := &openwrt.OpenwrtClientInfo{pod.Status.PodIP, "root", ""}
		runtime_instance, _ := handler.GetObject(clientInfo, handler.GetName(instance))
		// runtimePolicy, _ := mwan3.GetPolicy(mwan3Policy.Name)
		if runtime_instance == nil {
			reqLogger.Info("Runtime instance doesn't exist, so don't have to delete")
		} else {
			// err = mwan3.DeletePolicy(mwan3Policy.Name)
			err = handler.DeleteObject(clientInfo, handler.GetName(instance))
			if err != nil {
				reqLogger.Error(err, "Failed to delete instance")
				return false, err
			}
			// _, err = service.ExecuteService("mwan3", "restart")
			_, err = handler.Restart(clientInfo)
			if err != nil {
				reqLogger.Error(err, "Failed to restart openwrt service")
				return false, err
			}
			cnfChanged = true
		}
	}
	// We say the deletioni succeed only when the deletion for all pods succeed
	return cnfChanged, nil
}
