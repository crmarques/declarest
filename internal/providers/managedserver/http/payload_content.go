package http

import "github.com/crmarques/declarest/resource"

func unwrapContentValue(value any) any {
	if content, ok := value.(resource.Content); ok {
		return content.Value
	}
	return value
}

func unwrapContentDescriptor(value any) resource.PayloadDescriptor {
	if content, ok := value.(resource.Content); ok {
		return content.Descriptor
	}
	return resource.PayloadDescriptor{}
}
