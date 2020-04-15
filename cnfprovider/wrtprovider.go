package cnfprovider

import (
	"fmt"
	"reflect"
	sdewanv1alpha1 "sdewan.akraino.org/sdewan/api/v1alpha1"
	"sdewan.akraino.org/sdewan/openwrt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("wrt_provider")

type WrtProvider struct {
	Namespace string
	SdewanPurpose string
	Deployment corev1.Deployment
	K8sClient client.Client
}

func NetworkInterfaceMap(instance *sdewanv1alpha1.Sdewan) map[string]string {
	ifMap := make(map[string]string)
	for i, network := range instance.Spec.Networks {
		prefix := "lan_"
		if network.IsProvider {
			prefix = "wan_"
		}
		if network.Interface == "" {
			network.Interface = fmt.Sprintf("net%d", i)
		}
		ifMap[network.Name] = prefix + fmt.Sprintf("net%d", i)
	}
	return ifMap
}

func (p *Wrtprovider) net2iface(net string) string, error {
	type Iface struct{
		DefaultGateway bool
		Interface string
		Name string
	}
	type NfnNet struct{
		Type string
		Interface []Iface
	}
	ann := p.Deployment.Spec.Templete.Annotations
	nfnNet = NfnNet{}
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

func (p *Wrtprovider) convertCrd(mwan3Policy *sdewanv1alpha1.Mwan3Policy) openwrt.SdewanPolicy, error {
	members := make([]openwrt.SdewanMember, len(mwan3Policy.Spec.Members))
	for i, membercr := range mwan3Policy.Spec.Members {
		iface, err := p.net2iface(membercr.Network)
		if err != nil {
			return nil, err
		}
		members[i] = openwrt.SdewanMember{
			Interface: iface,
			Metric: membercr.Metric,
			Weight: membercr.Weight
		}
	}
	return openwrt.SdewanPolicy{Name: mwan3Policy.Name, Members: members}, nil

}

func (p *Wrtprovider) AddUpdateMwan3Policy(mwan3Policy *sdewanv1alpha1.Mwan3Policy) error {
        reqLogger := log.WithValues("Mwan3Policy", mwan3Policy.Name, "cnf", deploy.Name)
	ctx := context.Background()
	podList := &corev1.PodList{}
	err := p.K8sClient.Get(ctx, podList, client.MatchingLabels{"sdewanPurpose": p.SdewanPurpose})
	if err != nil {
		reqLogger.Error(err)
		return err
	}
	policy, err := p.convertCrd(mwan3Policy)
	if err != nil {
		reqLogger.Error(err, "Failed to convert mwan3Policy CR")
		return err
	}
	for _, pod := range podList.Items {
		openwrtClient := openwrt.NewOpenwrtClient(pod.Status.PodIP, "root", "")
		mwan3 := openwrt.Mwan3Client{OpenwrtClient: openwrtClient}
		service := openwrt.ServiceClient{OpenwrtClient: openwrtClient}
		runtimePolicy, _ := mwan3.GetPolicy(policy.Name)
		if runtimePolicy == nil {
			_, err := mwan3.CreatePolicy(policy)
			if err != nil {
				reqLogger.Error(err, "Failed to create policy")
				return err
			}
		} else if reflect.deepEqual(*runtimePolicy, policy) {
			reqLogger.Debug("Equal to the runtime policy, so no update")
		} else {
			_, err := mwan3.UpdatePolicy(policy)
			if err != nil {
				reqLogger.Error(err, "Failed to update policy")
				return err
			}
		}
	}
	// We say the AddUpdate succeed only when the add/update for all pods succeed
	return nil
}

func (p *Wrtprovider) DeleteMwan3Policy(mwan3Policy *sdewanv1alpha1.Mwan3Policy) error {
        reqLogger := log.WithValues("Mwan3Policy", mwan3Policy.Name, "cnf", deploy.Name)
        ctx := context.Background()
        podList := &corev1.PodList{}
        err := p.K8sClient.Get(ctx, podList, client.MatchingLabels{"sdewanPurpose": p.SdewanPurpose})
        if err != nil {
                reqLogger.Error(err, "Failed to get pod list")
                return err
        }
        for _, pod := range podList.Items {
                openwrtClient := openwrt.NewOpenwrtClient(pod.Status.PodIP, "root", "")
                mwan3 := openwrt.Mwan3Client{OpenwrtClient: openwrtClient}
                service := openwrt.ServiceClient{OpenwrtClient: openwrtClient}
                runtimePolicy, _ := mwan3.GetPolicy(mwan3Policy.Name)
                if runtimePolicy == nil {
                        reqLogger.Debug("Runtime policy doesn't exist, so don't have to delete")
                } else {
                        _, err := mwan3.DeletePolicy(mwan3Policy.Name)
                        if err != nil {
                                reqLogger.Error(err, "Failed to delete policy")
                                return err
                        }
                }
        }
        // We say the deletioni succeed only when the deletion for all pods succeed
        return nil
}

