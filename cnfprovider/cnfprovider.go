package cnfprovider

import (
	"fmt"
	"reflect"
	sdewanv1alpha1 "sdewan.akraino.org/sdewan/api/v1alpha1"
	"sdewan.akraino.org/sdewan/openwrt"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type CnfProvider interface {
	AddUpdateMwan3Policy(*sdewanv1alpha1.Mwan3Policy) error
	DeleteMwan3Policy(*sdewanv1alpha1.Mwan3Policy) error
	// TODO: Add more Interfaces here
	IsCnfReady() (bool, error)
}
