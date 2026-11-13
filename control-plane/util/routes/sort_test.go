package routes

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"strings"
	"testing"
)

func TestRouteMatcherLess(t *testing.T) {
	r1 := &domain.Route{
		Regexp: "/api/v1/([^/]+)",
	}
	r2 := &domain.Route{
		Regexp: "/api/v1/([^/]+)/users",
	}

	r1LessThanR2 := routeMatcherLess(r1, r2)
	assert.False(t, r1LessThanR2)

	r3 := &domain.Route{
		Prefix: "/api/v1/prefix/pre/last",
	}
	r4 := &domain.Route{
		Prefix: "/api/v1/prefix/pre/la",
	}

	r1LessThanR2 = routeMatcherLess(r3, r4)
	assert.True(t, r1LessThanR2)
}

func TestOrderRoutesForEnvoy_RegexpVersusRegexp(t *testing.T) {
	routes := []*domain.Route{
		{
			Regexp: "/api/v1/([^/]+)/users(/.*)?",
		},
		{
			Regexp: "/api/v1(/.*)?",
		},
		{
			Regexp: "/api/v1/([^/]+)(/.*)?",
		},
		{
			Regexp: "/api/v1/([^/]+)/users/([^/]+)(/.*)?",
		},
	}
	sorted := OrderRoutesForEnvoy(routes)

	assert.Len(t, sorted, len(routes))
	assert.Equal(t, routes[3].Regexp, sorted[0].Regexp)
	assert.Equal(t, routes[0].Regexp, sorted[1].Regexp)
	assert.Equal(t, routes[2].Regexp, sorted[2].Regexp)
	assert.Equal(t, routes[1].Regexp, sorted[3].Regexp)
}

func TestOrderRoutesForEnvoy_PrefixVersusPrefix(t *testing.T) {
	routes := []*domain.Route{
		{
			Prefix: "/api/v1/prefix/pre/last",
		},
		{
			Prefix: "/api/v1/prefix/pre/la",
		},
		{
			Prefix: "/api/v1/prefix/pre/last/final",
		},
		{
			Prefix: "/api/v1/prefix",
		},
		{
			Prefix: "/api",
		},
	}
	sorted := OrderRoutesForEnvoy(routes)

	assert.Len(t, sorted, len(routes))
	assert.Equal(t, routes[2], sorted[0])
	assert.Equal(t, routes[0], sorted[1])
	assert.Equal(t, routes[1], sorted[2])
	assert.Equal(t, routes[3], sorted[3])
	assert.Equal(t, routes[4], sorted[4])
}

func TestOrderRoutesForEnvoy_PSUPCLFRM1352(t *testing.T) {
	lessRoutePrefix := "/api/v1/tenants/6b741a74-5d2e-4dfe-90ea-e44f38f2bf10/shopping-frontend/"
	moreRoutePrefix := "/api/v1/tenants/6b741a74-5d2e-4dfe-90ea-e44f38f2bf10/shopping-frontend"

	routes := []*domain.Route{
		{Prefix: "/api/v1/routes", Regexp: ""}, // 20
		{Prefix: "", Regexp: "/api/v4/tenant-manager/activate/create-os-tenant-alias-routes/rollback/([^/]+)(/.*)?"},           // 78
		{Prefix: "/api/v1/tenants/6b741a74-5d2e-4dfe-90ea-e44f38f2bf10/shopping-frontend/", Regexp: ""},                        // 77
		{Prefix: "/api/v1/paas-mediation/namespaces/cloudbss311-platform-core-support-dev3/configmaps/bg-version", Regexp: ""}, // 100
		{Prefix: "", Regexp: "/api/v4/tenant-manager/resume/restore-os-tenant-alias-routes/perform/([^/]+)(/.*)?"},             // 76
		{Prefix: "/api/v2/control-plane/routing/details", Regexp: ""},                                                          // 43
		{Prefix: "/", Regexp: ""}, // 7
		{Prefix: "", Regexp: "/api/v4/tenant-manager/suspend/deactivate-os-tenant-alias-routes/rollback/([^/]+)(/.*)?"}, // 81
		{Prefix: "/api/v1/tenants/6b741a74-5d2e-4dfe-90ea-e44f38f2bf10/shopping-frontend", Regexp: ""},                  // 76
	}

	routePermuts := GenerateRoutePermutations(routes)

	// Check subset of permutations cuz there are too many of them (8! ~= 40000)
	for i := 0; i < 100; i++ {
		routePermut := routePermuts[rand.Int()%len(routePermuts)]
		func(permut []*domain.Route) {
			sorted := OrderRoutesForEnvoy(permut)
			assert.Len(t, sorted, len(routes))
			lessRouteIdx := getRouteIdx(sorted, lessRoutePrefix)
			assert.True(t, lessRouteIdx >= 0, "Cannot find route with lessRouteIdx")
			moreRouteIdx := getRouteIdx(sorted, moreRoutePrefix)
			assert.True(t, moreRouteIdx >= 0, "Cannot find route with moreRouteIdx")
			assert.True(t, lessRouteIdx < moreRouteIdx, "Routes order incorrect")
		}(routePermut)
	}
}

func TestOrderRoutesForEnvoy_PrefixVersusRegexp(t *testing.T) {
	routes := []*domain.Route{
		{
			Regexp: "/api/v1/ext-frontend-api/customers/import/([^/]+)(/.*)?",
		},
		{
			Regexp: "/api/v1/ext-frontend-api/customers/([^/]+)(/.*)?",
		},
		{
			Prefix: "/api/v1/ext-frontend-api/customers/import",
		},
		{
			Regexp: "/api/v1/ext-frontend-api/([^/]+)(/.*)?",
		},
		{
			Prefix: "/api/v1/ext-frontend-api/customers",
		},
		{
			Prefix: "/api/v1/ext-frontend-api/method",
		},
		{
			Prefix: "/api/v1/ext-frontend-api",
		},
		{
			Regexp: "/api/v1/ext-frontend-api/(/.*)?",
		},
	}
	routePermuts := GenerateRoutePermutations(routes)
	for _, routePermut := range routePermuts {
		go func(routePermut []*domain.Route) {
			sorted := OrderRoutesForEnvoy(routePermut)
			assert.Len(t, sorted, len(routes))

			assert.Equal(t, routes[0], sorted[0], "%s %s must be at 0 index", routes[0].Prefix, routes[0].Regexp)
			assert.Equal(t, routes[2], sorted[1], "%s %s must be at 1 index", routes[1].Prefix, routes[1].Regexp)
			assert.Equal(t, routes[1], sorted[2], "%s %s must be at 2 index", routes[2].Prefix, routes[2].Regexp)
			assert.Equal(t, routes[4], sorted[3], "%s %s must be at 3 index", routes[3].Prefix, routes[3].Regexp)
			assert.Equal(t, routes[5], sorted[4], "%s %s must be at 4 index", routes[4].Prefix, routes[4].Regexp)
			assert.Equal(t, routes[3], sorted[5], "%s %s must be at 5 index", routes[5].Prefix, routes[5].Regexp)
			assert.Equal(t, routes[7], sorted[6], "%s %s must be at 6 index", routes[6].Prefix, routes[6].Regexp)
			assert.Equal(t, routes[6], sorted[7], "%s %s must be at 7 index", routes[7].Prefix, routes[7].Regexp)
		}(routePermut)
	}
}

func TestOrderRoutesForEnvoy_PSUPCLFRM1915(t *testing.T) {
	lessRoutePrefix := "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend/"
	moreRoutePrefix := "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend"

	routes := []*domain.Route{
		{Prefix: "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend", Regexp: ""},
		{Prefix: "", Regexp: `\\/api\\/v1\\/opportunity-management\\/customer-contacts\\/layout\\/([^/]+)\\/([^/]+)\\/([^/]+)(/.*)?`},
		{Prefix: "", Regexp: `\\/api\\/v3\\/tenant-manager\\/activate\\/create-os-tenant-alias-routes\\/rollback\\/([^/]+)(/.*)?`},
		{Prefix: "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend/", Regexp: ""},
	}

	routePermuts := GenerateRoutePermutations(routes)

	for _, routePermut := range routePermuts {
		func(permut []*domain.Route) {
			sorted := OrderRoutesForEnvoy(permut)
			assert.Len(t, sorted, len(routes))
			lessRouteIdx := getRouteIdx(sorted, lessRoutePrefix)
			assert.True(t, lessRouteIdx >= 0, "Cannot find route with lessRouteIdx")
			moreRouteIdx := getRouteIdx(sorted, moreRoutePrefix)
			assert.True(t, moreRouteIdx >= 0, "Cannot find route with moreRouteIdx")
			assert.True(t, lessRouteIdx < moreRouteIdx, "Routes order incorrect")
			assertRouteOrder(t, sorted)
		}(routePermut)
	}
}

func TestRouteMatcher_TransitiveRelation(t *testing.T) {
	r1 := &domain.Route{
		Prefix: "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend/", // 77
	}
	r2 := &domain.Route{
		Prefix: "/api/v1/tenants/342e3f64-77f6-4d87-9615-324f99f896d0/shopping-frontend", // 76
	}
	r3 := &domain.Route{
		Regexp: "/api/v1/customer-api-internal/subscribers-and-groups-report/([^/]+)/feedback(/.*)?", // 76
	}

	// Check if r1 < r2 and r2 < r3 then r1 < r3
	firstHigherInRoutingTable := routeMatcherLess(r1, r2)
	assert.True(t, firstHigherInRoutingTable)
	firstHigherInRoutingTable = routeMatcherLess(r2, r3)
	assert.True(t, firstHigherInRoutingTable)
	firstHigherInRoutingTable = routeMatcherLess(r1, r3)
	assert.True(t, firstHigherInRoutingTable)

	// Check if r2 > r1 and r3 > r2 then r3 > r1
	firstHigherInRoutingTable = routeMatcherLess(r2, r1)
	assert.False(t, firstHigherInRoutingTable)
	firstHigherInRoutingTable = routeMatcherLess(r3, r2)
	assert.False(t, firstHigherInRoutingTable)
	firstHigherInRoutingTable = routeMatcherLess(r3, r1)
	assert.False(t, firstHigherInRoutingTable)
}

func TestRouteMatcher_EqualPriorityButUndefinedOrder(t *testing.T) {
	r1 := &domain.Route{
		Prefix: "/api/v1/route/a",
	}
	r2 := &domain.Route{
		Prefix: "/api/v1/route/b",
	}
	firstComparison := routeMatcherLess(r1, r2)
	secondComparison := routeMatcherLess(r2, r1)

	assert.False(t, firstComparison)  // means that r2 > r1
	assert.False(t, secondComparison) // means that r1 > r2

	r3 := &domain.Route{
		Regexp: "/api/v1/route/([^/]+)(/.*)?",
	}
	r4 := &domain.Route{
		Regexp: "/api/v2/route/([^/]+)(/.*)?",
	}
	firstComparison = routeMatcherLess(r3, r4)
	secondComparison = routeMatcherLess(r4, r3)

	assert.False(t, firstComparison)  // means that r4 > r3
	assert.False(t, secondComparison) // means that r3 > r4
}

func TestRouteMatcher_EqualPriority_PrefixPrioritizedOverRegexp(t *testing.T) {
	r1 := &domain.Route{
		Prefix: "/api/v1/route/a",
	}
	r2 := &domain.Route{
		Regexp: "/api/v1/route/([^/]+)(/.*)?",
	}
	less := routeMatcherLess(r1, r2)
	assert.True(t, less)

	less = routeMatcherLess(r2, r1)
	assert.False(t, less)
}

func TestRouteMatcher_DenyRoutes(t *testing.T) {
	longAllow := &domain.Route{Regexp: "/api/v6/quote/([^/]+)(/.*)?"}
	shortDeny := &domain.Route{Prefix: "/api/v6/q", Deny: true}
	longDeny := &domain.Route{Prefix: "/api/v6/quote/internal/operations", Deny: true}

	assert.True(t, routeMatcherLess(shortDeny, longAllow))
	assert.False(t, routeMatcherLess(longAllow, shortDeny))

	assert.True(t, routeMatcherLess(longDeny, shortDeny))
	assert.False(t, routeMatcherLess(shortDeny, longDeny))

	assert.False(t, routeMatcherLess(shortDeny, shortDeny))
}

func TestRouteMatcher_StrictWeakOrderingWithDenyRoutes(t *testing.T) {
	routes := []*domain.Route{
		{Prefix: "/api/v1/route/a"},
		{Prefix: "/api/v1/route/b"},
		{Regexp: "/api/v1/route/([^/]+)(/.*)?"},
		{Regexp: "/api/v1/route/([^/]+)(/.*)?", Deny: true},
		{Prefix: "/api/v1/route/c", Deny: true},
		{Prefix: "/api/v1/route/c", HeaderMatchers: []*domain.HeaderMatcher{{Name: "x-custom"}}},
		{Prefix: "/api/v1/route/c", HeaderMatchers: []*domain.HeaderMatcher{{Name: "x-custom"}}, Deny: true},
		{Prefix: "/api", Deny: true},
		{Prefix: "/api/v1/route/very/long/path"},
		{Prefix: "/api/v1/route/very/long/path/deny", Deny: true},
		{Regexp: "/api/v1/([^/]+)/users/([^/]+)(/.*)?"},
	}
	for _, a := range routes {
		assert.False(t, routeMatcherLess(a, a), "irreflexivity: %+v", a)
		for _, b := range routes {
			if routeMatcherLess(a, b) {
				assert.False(t, routeMatcherLess(b, a), "asymmetry: %+v vs %+v", a, b)
			}
			for _, c := range routes {
				if routeMatcherLess(a, b) && routeMatcherLess(b, c) {
					assert.True(t, routeMatcherLess(a, c), "transitivity: %+v < %+v < %+v", a, b, c)
				}
				if !routeMatcherLess(a, b) && !routeMatcherLess(b, a) && !routeMatcherLess(b, c) && !routeMatcherLess(c, b) {
					assert.False(t, routeMatcherLess(a, c) || routeMatcherLess(c, a),
						"transitivity of equivalence: %+v ~ %+v ~ %+v", a, b, c)
				}
			}
		}
	}
}

func TestOrderRoutesForEnvoy_DenyRoutesPermutations(t *testing.T) {
	routes := []*domain.Route{
		{Regexp: "/test-service/quoteManagement/v6/quote/([^/]+)(/.*)?"},
		{Prefix: "/test-service/quoteManagement/v6/quote/"},
		{Prefix: "/test-service/quoteManagement/v6/quote"},
		{Prefix: "/test-service/quoteManagement/v6"},
		{Prefix: "/test-service/quoteManagement/v6/hidden", Deny: true},
		{Prefix: "/test-service/q", Deny: true},
	}
	expected := []*domain.Route{routes[4], routes[5], routes[0], routes[1], routes[2], routes[3]}

	for _, permut := range GenerateRoutePermutations(routes) {
		assert.Equal(t, expected, OrderRoutesForEnvoy(permut))
	}
}

func TestOrderRoutesForEnvoy_DenyRoutesLargeSet(t *testing.T) {
	quoteRegexp := &domain.Route{Regexp: "/test-service/quoteManagement/v6/quote/([^/]+)(/.*)?"}
	quotePrefix := &domain.Route{Prefix: "/test-service/quoteManagement/v6/quote"}
	routes := []*domain.Route{quoteRegexp, quotePrefix, {Prefix: "/test-service/quoteManagement/v6/quote/"}}
	for i := 0; i < 60; i++ {
		routes = append(routes, &domain.Route{Prefix: fmt.Sprintf("/svc-%d/api/v1/%s", i, strings.Repeat("x", i%25))})
	}
	for i := 0; i < 3; i++ {
		routes = append(routes, &domain.Route{Prefix: fmt.Sprintf("/test-service/internal-%d", i), Deny: true})
	}

	rnd := rand.New(rand.NewSource(1))
	for run := 0; run < 200; run++ {
		permut := make([]*domain.Route, len(routes))
		copy(permut, routes)
		rnd.Shuffle(len(permut), func(i, j int) { permut[i], permut[j] = permut[j], permut[i] })

		sorted := OrderRoutesForEnvoy(permut)
		assert.Len(t, sorted, len(routes))
		for i := 0; i < 3; i++ {
			assert.True(t, sorted[i].Deny, "deny routes must go first")
		}
		for i := range sorted {
			for j := i + 1; j < len(sorted); j++ {
				assert.False(t, routeMatcherLess(sorted[j], sorted[i]), "%+v must not go after %+v", sorted[j], sorted[i])
			}
		}
		assert.Less(t, routeIdx(sorted, quoteRegexp), routeIdx(sorted, quotePrefix))
	}
}

func routeIdx(routes []*domain.Route, route *domain.Route) int {
	for i, r := range routes {
		if r == route {
			return i
		}
	}
	return -1
}

// assertRouteOrder assumes that there are no equal priorities in sortedRoutes param
func assertRouteOrder(t *testing.T, sortedRoutes []*domain.Route) {
	for i := 0; i < len(sortedRoutes); i++ {
		for j := 0; j < len(sortedRoutes); j++ {
			if j < i {
				assert.True(t, routeMatcherLess(sortedRoutes[j], sortedRoutes[i]))
			}
			if j > i {
				assert.True(t, routeMatcherLess(sortedRoutes[i], sortedRoutes[j]))
			}
		}
	}
}

func getRouteIdx(routes []*domain.Route, routePrefix string) int {
	for i, r := range routes {
		if r.Prefix == routePrefix {
			return i
		}
	}
	return -1
}

func GenerateRoutePermutations(routes []*domain.Route) [][]*domain.Route {
	var oldPermuts [][]*domain.Route
	routesCpy := make([]*domain.Route, len(routes))
	copy(routesCpy, routes)

	for i := 0; i < len(routesCpy)-1; i++ {
		if i == 0 {
			oldPermuts = getPermutationsForSubArr(routesCpy[0:1], routesCpy[i+1])
			continue
		}
		currPermuts := make([][]*domain.Route, 0)
		for j, _ := range oldPermuts {
			currPermuts = append(currPermuts, getPermutationsForSubArr(oldPermuts[j], routesCpy[i+1])...)
		}
		oldPermuts = currPermuts
	}
	return oldPermuts
}

func getPermutationsForSubArr(constructing []*domain.Route, toAdd *domain.Route) [][]*domain.Route {
	permuts := make([][]*domain.Route, len(constructing)+1)
	for i := 0; i <= len(constructing); i++ {
		permut := make([]*domain.Route, len(constructing))
		copy(permut, constructing)
		permuts[i] = insert(permut, i, toAdd)
	}
	return permuts
}

func insert(arr []*domain.Route, index int, value *domain.Route) []*domain.Route {
	if len(arr) == index { // nil or empty slice or after last element
		return append(arr, value)
	}
	arr = append(arr[:index+1], arr[index:]...) // index < len(arr)
	arr[index] = value
	return arr
}
