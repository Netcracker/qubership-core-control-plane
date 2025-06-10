package routes

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"sort"
	"strings"
)

const minimalGroupCaptured = "A" // can be any string with len == 1

func routeMatcherLess(r1, r2 *domain.Route) bool {
	r1HeadersLen := 0
	if r1.HeaderMatchers != nil {
		r1HeadersLen = len(r1.HeaderMatchers)
	}
	r2HeadersLen := 0
	if r2.HeaderMatchers != nil {
		r2HeadersLen = len(r2.HeaderMatchers)
	}
	h := r1HeadersLen - r2HeadersLen

	comparableR1, comparableR2 := comparisonTarget(*r1), comparisonTarget(*r2)
	r := len(comparableR1.GetComparisonString()) - len(comparableR2.GetComparisonString())

	if r == 0 {
		if r == h {
			if comparableR1.matchesByPermissions() {
				return true
			}
			// Matching by prefix is more prioritized over regexp
			if comparableR1.matchesByPrefix() && comparableR2.matchesByRegexp() {
				return true
			}
			if comparableR1.matchesByRegexp() && comparableR2.matchesByPrefix() {
				return false
			}
			// Undefined priority case
			return false
		} else {
			return h > 0
		}
	}
	return r > 0
}

type comparisonTarget domain.Route

func (r comparisonTarget) GetComparisonString() string {
	if r.matchesByRegexp() {
		return replaceGroupCapturesWithMinimalMatching(removeDoubleBackslashes(r.Regexp))
	}
	if r.matchesByPrefix() {
		return removeDoubleBackslashes(r.getPrefixAsRegexp())
	}
	return ""
}

func (r comparisonTarget) getPrefixAsRegexp() string {
	return r.Prefix + "(/.*)?"
}

func (r comparisonTarget) matchesByRegexp() bool {
	return len(r.Regexp) > 0
}

func (r comparisonTarget) matchesByPrefix() bool {
	return len(r.Prefix) > 0
}

func (r comparisonTarget) matchesByPermissions() bool {
	return r.DirectResponseCode == 404
}

func replaceGroupCapturesWithMinimalMatching(str string) string {
	return strings.ReplaceAll(str, "([^/]+)", minimalGroupCaptured)
}

func removeDoubleBackslashes(str string) string {
	return strings.ReplaceAll(str, "\\", "")
}

func OrderRoutesForEnvoy(routes []*domain.Route) (sortedRoutes []*domain.Route) {
	balancingRoutes := make([]*domain.Route, 0)
	xVersionRoutes := make([]*domain.Route, 0)
	otherRoutes := make([]*domain.Route, 0)
	for _, route := range routes {
		// search for balancing routes, they must go first
		if route.HeaderMatchers != nil {
			for _, matcher := range route.HeaderMatchers {
				if strings.EqualFold("x-anchor", matcher.Name) {
					balancingRoutes = append(balancingRoutes, route)
					break
				}
			}
		}
		hasXVersionHeader := false
		if route.HeaderMatchers != nil {
			for _, matcher := range route.HeaderMatchers {
				if strings.EqualFold("x-version", matcher.Name) {
					hasXVersionHeader = true
					break
				}
			}
		}
		if hasXVersionHeader {
			xVersionRoutes = append(xVersionRoutes, route)
		} else {
			otherRoutes = append(otherRoutes, route)
		}
	}
	sort.SliceStable(balancingRoutes, func(i, j int) bool {
		return routeMatcherLess(balancingRoutes[i], balancingRoutes[j])
	})
	sort.SliceStable(xVersionRoutes, func(i, j int) bool {
		return routeMatcherLess(xVersionRoutes[i], xVersionRoutes[j])
	})
	sort.SliceStable(otherRoutes, func(i, j int) bool {
		return routeMatcherLess(otherRoutes[i], otherRoutes[j])
	})
	sortedRoutes = append(sortedRoutes, balancingRoutes...)
	sortedRoutes = append(sortedRoutes, xVersionRoutes...)
	sortedRoutes = append(sortedRoutes, otherRoutes...)
	return
}
