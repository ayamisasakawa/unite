package main

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"gocv.io/x/gocv"

	"github.com/pidgy/unitehud/team"
)

type filter struct {
	*team.Team
	file   string
	value  int
	scalar float64
}

type template struct {
	filter
	gocv.Mat
	category    string
	subcategory string
}

var (
	filenames map[string]map[string][]filter
	templates map[string]map[string][]template
)

func load() {
	for category := range filenames {
		for subcategory, filters := range filenames[category] {
			for _, filter := range filters {
				templates[category][filter.Team.Name] = append(templates[category][filter.Team.Name],
					template{
						filter,
						gocv.IMRead(filter.file, gocv.IMReadColor),
						category,
						subcategory,
					},
				)
			}
		}
	}

	for category := range templates {
		for _, templates := range templates[category] {
			for _, t := range templates {
				if t.Empty() {
					kill(fmt.Errorf("invalid scored template: %s (scale: %.2f)", t.file, t.scalar))
				}

				log.Debug().Object("template", t).Msg("score template loaded")
			}
		}
	}
}
