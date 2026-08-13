package domain

import "github.com/puzpuzpuz/xsync/v4"

var (
	runtimeIdByNameGen = newRuntimeIdByNameGenerator()
)

func newRuntimeIdByNameGenerator() runtimeIdByNameGenerator {
	return runtimeIdByNameGenerator{idsMap: xsync.NewMap[string, int32]()}
}

type runtimeIdByNameGenerator struct {
	lastId int32
	idsMap *xsync.Map[string, int32]
}

func (g *runtimeIdByNameGenerator) GetIdByName(name string) int32 {
	res, _ := g.idsMap.LoadOrCompute(name, func() (int32, bool) {
		g.lastId = g.lastId + 1
		return g.lastId, false
	})
	return res
}
