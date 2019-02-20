package config

import (
	"sync"

	"github.com/aws/aws-sdk-go/service/rds"
)

// Backend ...
type Backend interface {
	CatalogReader(state *CatalogState, nodeName string, quitCh chan int)
	WriteService(service *Service)
	DeleteCheck(check, node string)
	DeleteService(service, node string)
}

// Config ...
type Config struct {
	ConsulNodeName      string
	ConsulRDSMasterTag  string
	ConsulRDSReplicaTag string
}

// SeenCatalog ...
type SeenCatalog struct {
	Services []string
	Checks   []string
}

// Tags ...
type Tags map[string]string

// DBInstance ...
type DBInstance struct {
	*rds.DBInstance
	Tags Tags
}

// Filters ...
type Filters map[string]string

// Service ...
type Service struct {
	ServiceID      string
	ServiceName    string
	ServiceAddress string
	ServicePort    int
	ServiceTags    []string
	ServiceMeta    map[string]string
	CheckID        string
	CheckNode      string
	CheckNotes     string
	CheckStatus    string
	CheckOutput    string
}

// Services ...
type Services map[string]*Service

func (s Services) GetSeen() SeenCatalog {
	seen := SeenCatalog{
		Services: make([]string, 0),
		Checks:   make([]string, 0),
	}

	for _, service := range s {
		seen.Checks = append(seen.Checks, service.CheckID)
		seen.Services = append(seen.Services, service.ServiceID)
	}

	return seen
}

// CatalogState ...
type CatalogState struct {
	Services Services
	sync.Mutex
}
