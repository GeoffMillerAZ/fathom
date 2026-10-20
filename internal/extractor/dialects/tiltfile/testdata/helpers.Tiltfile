"""Shared helpers."""

def image_name(service, tag = "dev"):
    """The image reference of a service."""
    return "example/%s:%s" % (service, tag)

def with_port(resource, port):
    k8s_resource(resource, port_forwards = [port])
    return resource

def noop():
    pass
