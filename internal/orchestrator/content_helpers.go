package orchestrator

import "github.com/crmarques/declarest/resource"

func contentFromResource(value resource.Resource) resource.Content {
	return resource.Content{
		Value:      value.Payload,
		Descriptor: value.PayloadDescriptor,
	}
}

func contentWithDescriptor(value any, descriptor resource.PayloadDescriptor) resource.Content {
	return resource.Content{
		Value:      value,
		Descriptor: descriptor,
	}
}

func requestBodyPresent(body resource.Content) bool {
	return body.Value != nil
}
