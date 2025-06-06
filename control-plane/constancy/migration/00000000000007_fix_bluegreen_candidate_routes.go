package migration

import (
	"context"
	"database/sql"
	"github.com/pkg/errors"
	"github.com/uptrace/bun"
	"strings"
	"time"
)

func init() {
	migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			log.Infof("#7 Fix bluegreen candidates routes")
			candidateVersions, err := getVersionsByStage(ctx, &tx, "CANDIDATE")
			if err != nil {
				return err
			}
			if len(candidateVersions) == 0 {
				return nil
			}
			log.Debugf("Candidate versions: '%v'", candidateVersions)
			activeVersion, err := getActiveVersion(ctx, &tx)
			if err != nil {
				return err
			}
			groupedActiveRoutes, err := getRoutesGroupedByVirtualHostId(ctx, &tx, activeVersion)
			if err != nil {
				return err
			}
			for _, candidateVersion := range candidateVersions {
				groupedCandidateRoutes, err := getRoutesGroupedByVirtualHostId(ctx, &tx, candidateVersion)
				if err != nil {
					return err
				}
				for activeVirtualHostId, activeRoutes := range groupedActiveRoutes {
					if candidateRoutes, found := groupedCandidateRoutes[activeVirtualHostId]; found {
						for _, activeRoute := range activeRoutes {
							if !contains(candidateRoutes, activeRoute) {
								routeKey := strings.TrimSuffix(activeRoute.RouteKey, activeVersion) + candidateVersion
								forbiddenRoute := &v7RouteNew{
									VirtualHostId:            activeRoute.VirtualHostId,
									RouteKey:                 routeKey,
									Prefix:                   activeRoute.Prefix,
									Regexp:                   activeRoute.Regexp,
									DeploymentVersion:        candidateVersion,
									DirectResponseCode:       404,
									Version:                  0,
									InitialDeploymentVersion: candidateVersion,
									Autogenerated:            true,
								}
								_, err := tx.NewInsert().Model(forbiddenRoute).Exec(ctx)
								if err != nil {
									return errors.Wrapf(err, "Inserting prohibit route '%v' caused error", *forbiddenRoute)
								}
							}
						}
					}
				}
			}

			return nil
		})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}

func getVersionsByStage(ctx context.Context, tx *bun.Tx, stage string) ([]string, error) {
	var versions []string
	err := tx.NewSelect().Model((*v7DeploymentVersion)(nil)).Column("version").Scan(ctx, &versions)
	if err != nil {
		return nil, errors.Wrapf(err, "Finding all deployments for stage '%s' has failed", stage)
	}
	return versions, nil
}

func getActiveVersion(ctx context.Context, tx *bun.Tx) (string, error) {
	activeVersions, err := getVersionsByStage(ctx, tx, "ACTIVE")
	if err != nil {
		return "", err
	}
	if len(activeVersions) > 0 {
		return activeVersions[0], nil
	} else {
		return "v1", nil
	}
}

func getRoutesGroupedByVirtualHostId(ctx context.Context, tx *bun.Tx, version string) (map[int32][]v7Route, error) {
	var routes []v7Route
	err := tx.NewSelect().Model(&routes).Where("deployment_version = ?", version).Scan(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "Finding routes by version '%s' has failed", version)
	}
	result := make(map[int32][]v7Route)
	for _, route := range routes {
		if groupedRoutes, found := result[route.VirtualHostId]; found {
			result[route.VirtualHostId] = append(groupedRoutes, route)
		} else {
			result[route.VirtualHostId] = []v7Route{route}
		}
	}
	return result, nil
}

type v7DeploymentVersion struct {
	bun.BaseModel `bun:"table:deployment_versions"`
	Version       string    `bun:",pk" json:"version"`
	Stage         string    `bun:",notnull" json:"stage"`
	CreatedWhen   time.Time `bun:"createdwhen,nullzero,notnull,default:current_timestamp" json:"createdWhen"`
	UpdatedWhen   time.Time `bun:"updatedwhen,nullzero,notnull,default:current_timestamp" json:"updatedWhen"`
}

type v7Route struct {
	bun.BaseModel `bun:"table:routes"`
	VirtualHostId int32  `bun:"virtualhostid,notnull"`
	RouteKey      string `bun:"routekey,notnull"`
	Prefix        string `bun:"rm_prefix"`
	Regexp        string `bun:"rm_regexp"`
}

type v7RouteNew struct {
	bun.BaseModel            `bun:"routes"`
	Id                       int32
	VirtualHostId            int32  `bun:"virtualhostid,notnull"`
	RouteKey                 string `bun:"routekey,notnull"`
	Prefix                   string `bun:"rm_prefix"`
	Regexp                   string `bun:"rm_regexp"`
	DeploymentVersion        string `bun:"deployment_version,notnull"`
	DirectResponseCode       uint32 `bun:"directresponse_status"`
	Version                  int32  `bun:",notnull"`
	InitialDeploymentVersion string `bun:"initialdeploymentversion,notnull"`
	Autogenerated            bool   `bun:"autogenerated,default:false"`
}
