package domain

const (
	PublicGateway      = "public-gateway-service"
	PrivateGateway     = "private-gateway-service"
	InternalGateway    = "internal-gateway-service"
	EgressGateway      = "egress-gateway"
	ProfilePublic      = "public"
	ProfilePrivate     = "private"
	ProfileInternal    = "internal"
	ExtAuthClusterName = "ext-authz"
)

func Gateways() map[string]string {
	return map[string]string{ProfilePublic: PublicGateway, ProfilePrivate: PrivateGateway, ProfileInternal: InternalGateway}
}

func Profiles() []string {
	return []string{ProfilePublic, ProfilePrivate, ProfileInternal}
}

func IsOobGateway(gateway string) bool {
	return gateway == InternalGateway || gateway == PrivateGateway || gateway == PublicGateway
}

func IsGatewayInternal(gateway string) bool {
	if gateway == InternalGateway {
		return true
	}
	return false
}

func IsGatewayPrivate(gateway string) bool {
	if gateway == PrivateGateway {
		return true
	}
	return false
}
