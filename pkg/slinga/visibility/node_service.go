package visibility

import (
	"fmt"
	"github.com/Frostman/aptomi/pkg/slinga"
)

type serviceNode struct {
	serviceName string
}

func newServiceNode(serviceName string) graphNode {
	return serviceNode{serviceName: serviceName}
}

func (n serviceNode) getIDPrefix() string {
	return "svc-"
}

func (n serviceNode) getGroup() string {
	return "service"
}

func (n serviceNode) getID() string {
	return fmt.Sprintf("%s%s", n.getIDPrefix(), n.serviceName)
}

func (n serviceNode) isItMyID(id string) string {
	return cutPrefixOrEmpty(id, n.getIDPrefix())
}

func (n serviceNode) getLabel() string {
	return n.serviceName
}

func (n serviceNode) getEdgeLabel(dst graphNode) string {
	// if it's an edge from service to service instance, write context information on it
	if dstInst, ok := dst.(serviceInstanceNode); ok {
		return fmt.Sprintf("%s/%s", dstInst.context, dstInst.allocation)
	}
	return ""
}

func (n serviceNode) getDetails(id string, state slinga.ServiceUsageState) interface{} {
	return state.Policy.Services[id]
}
