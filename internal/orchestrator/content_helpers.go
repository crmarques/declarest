package orchestrator

import "github.com/crmarques/declarest/resource"

func contentFromResource(value resource.Resource) resource.Content {
	return resource.Content{
		Value:      value.Payload,
		Descriptor: value.PayloadDescriptor,
	}
}

func requestBodyPresent(body resource.Content) bool {
	return body.Value != nil
}
