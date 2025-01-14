package elasticsearch

import (
	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/log"
)

func VerifyBinding(service config.Service) bool {
	valid := true

	if len(service.Binding.Host) == 0 {
		log.Errorf("service binding for [%s] is missing property: [host]", service.Name)
		valid = false
	}
	if len(service.Binding.Username) == 0 {
		log.Errorf("service binding for [%s] is missing property: [username]", service.Name)
		valid = false
	}
	if len(service.Binding.Password) == 0 {
		log.Errorf("service binding for [%s] is missing property: [password]", service.Name)
		valid = false
	}

	return valid
}
