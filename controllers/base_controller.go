package controllers

import (
	"context"
	"time"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1alpha1 "sdewan.akraino.org/sdewan/api/v1alpha1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"sdewan.akraino.org/sdewan/cnfprovider"
)

type ISdewanHandler interface {
    GetType() string
    GetName(instance runtime.Object)
    GetFinalizer() string
    GetInstance(r client.Client, ctx Context, req ctrl.Request) (runtime.Object, error)
    Convert(o runtime.Object, deployment extensionsv1beta1.Deployment) (IOpenWrtObject, error)
    IsEqual(instance1 IOpenWrtObject, instance2 IOpenWrtObject) (bool)
    GetObject(clientInfo *openwrt.OpenwrtClientInfo, name string) (IOpenWrtObject, error)
    CreateObject(clientInfo *OpenwrtClientInfo, instance IOpenWrtObject) (IOpenWrtObject, error)
    UpdateObject(clientInfo *OpenwrtClientInfo, instance IOpenWrtObject) (IOpenWrtObject, error)
    DeleteObject(clientInfo *OpenwrtClientInfo, name string) (error)
    Restart(clientInfo *OpenwrtClientInfo) (bool, error)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func getPurpose(instance runtime.Object) string {
    value := reflect.ValueOf(instance)
    field := reflect.Indirect(value).FieldByName("Labels")
    labels := field.Interface().(map[string]string)
    return labels["sdewanPurpose"]
}

func getDeletionTempstamp(instance runtime.Object) *time.Time {
	// to do: time.Time
    value := reflect.ValueOf(instance)
    field := reflect.Indirect(value).FieldByName("DeletionTimestamp")
    return field.Interface().(*time.Time)
}

func getFinalizers(instance runtime.Object) []string {
    value := reflect.ValueOf(instance)
    field := reflect.Indirect(value).FieldByName("Finalizers")
    return field.Interface().([]string)
}

func setStatus(instance runtime.Object, t *metav1.Time, isSync bool) {
    value := reflect.ValueOf(instance)
    field_rv := reflect.Indirect(value).FieldByName("ResourceVersion")
    rv := field_rv.Interface().(string)
    field_status := reflect.Indirect(value).FieldByName("Status")
    status := field_status.Interface().(SdewanStatus)
    status.AppliedVersion = rv
    status.AppliedTime = t
    status.InSync = isSync
    field_status.Set(reflect.ValueOf(status))
}

func appendFinalizer(instance runtime.Object, item string) {
	// to do: ObjectMeta
    value := reflect.ValueOf(instance)
    field := reflect.Indirect(value).FieldByName("ObjectMeta")
    base_obj := field.Interface().(ObjectMeta)
    base_obj.Finalizers = append(base_obj.Finalizers, item)
    field.Set(reflect.ValueOf(base_obj))
}

func removeFinalizer(instance runtime.Object, item string) {
    value := reflect.ValueOf(instance)
    field := reflect.Indirect(value).FieldByName("ObjectMeta")
    base_obj := field.Interface().(ObjectMeta)
    base_obj.Finalizers = removeString(base_obj.Finalizers, item)
    field.Set(reflect.ValueOf(base_obj))
}

func net2iface(net string, deployment extensionsv1beta1.Deployment) (string, error) {
	type Iface struct {
		DefaultGateway bool `json:"defaultGateway,string"`
		Interface      string
		Name           string
	}
	type NfnNet struct {
		Type      string
		Interface []Iface
	}
	ann := deployment.Spec.Template.Annotations
	nfnNet := NfnNet{}
	err := json.Unmarshal([]byte(ann["k8s.plugin.opnfv.org/nfn-network"]), &nfnNet)
	if err != nil {
		return "", err
	}
	for _, iface := range nfnNet.Interface {
		if iface.Name == net {
			return iface.Interface, nil
		}
	}
	return "", errors.New(fmt.Sprintf("No matched network in annotation: %s", net))

}

// Common Reconcile Processing
func ProcessReconcile(r client.Client, logger logr.Logge, req ctrl.Request, handler ISdewanHandler) (ctrl.Result, error) {
	ctx := context.Background()
	log := logger.WithValues(handler.GetType(), req.NamespacedName)

	// your logic here
	during, _ := time.ParseDuration("5s")

	//instance := &batchv1alpha1.Mwan3Policy{}
	//err := r.Get(ctx, req.NamespacedName, instance)
	instance, err := handler.GetInstance(r, ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			// No instance
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: during}, nil
	}
	// cnf, err := cnfprovider.NewWrt(req.NamespacedName.Namespace, instance.Labels["sdewanPurpose"], r.Client)
	// Labels: map[string]string
    purpose := getPurpose(instance)
	cnf, err := cnfprovider.NewOpenWrt(req.NamespacedName.Namespace, purpose, r)
	if err != nil {
		log.Error(err, "Failed to get cnf")
		// A new event are supposed to be received upon cnf ready
		// so not requeue
		return ctrl.Result{}, nil
	}
	// finalizerName := "rule.finalizers.sdewan.akraino.org"
	finalizerName := handler.GetFinalizer()
	// if instance.ObjectMeta.DeletionTimestamp.IsZero() {
	// DeletionTimestamp: *Time
    delete_timestamp := getDeletionTempstamp(instance)
	if delete_timestamp.IsZero() {
		// creating or updating CR
		if cnf == nil {
			// no cnf exists
			log.Info("No cnf exist, so not create/update " + handler.GetType())
			return ctrl.Result{}, nil
		}
		changed, err := cnf.AddOrUpdateObject(handler, instance)
		if err != nil {
			log.Error(err, "Failed to add/update " + handler.GetType())
			return ctrl.Result{RequeueAfter: during}, nil
		}
		// if !containsString(instance.ObjectMeta.Finalizers, finalizerName) {
		// Finalizers: []string
		finalizers := getFinalizers(instance)
		if !containsString(finalizers, finalizerName) {
			log.Info("Adding finalizer for " + handler.GetType())
			// instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)
			// Finalizers: []string
			appendFinalizer(instance, finalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
		if changed {
			// instance.Status.AppliedVersion = instance.ResourceVersion
			// instance.Status.AppliedTime = &metav1.Time{Time: time.Now()}
			// instance.Status.InSync = true
			// Status: SdewanStatus
			setStatus(instance, &metav1.Time{Time: time.Now()}, true)
			err = r.Status().Update(ctx, instance)
			if err != nil {
				log.Error(err, "Failed to update status for " + handler.GetType())
				return ctrl.Result{}, err
			}
		}
	} else {
		// deletin CR
		if cnf == nil {
			// no cnf exists
			finalizers := getFinalizers(instance)
			if containsString(finalizers, finalizerName) {
				// instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, finalizerName)
				removeFinalizer(instance, finalizerName)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, nil
		}
		//_, err := cnf.DeleteMwan3Policy(instance)
		_, err := cnf.DeleteObject(handler, instance)
		if err != nil {
			log.Error(err, "Failed to delete " + handler.GetType())
			return ctrl.Result{RequeueAfter: during}, nil
		}
		// if containsString(instance.ObjectMeta.Finalizers, finalizerName) {
		finalizers := getFinalizers(instance)
		if containsString(finalizers, finalizerName) {
			// instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, finalizerName)
			removeFinalizer(instance, finalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}