package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/language/apiv1/languagepb"
	"github.com/cyber-nic/gexf"
)

func main() {
	outFilename := "output.gexf"
	path := os.Args[1]
	start := time.Now()

	// Create a new GEXF graph
	g := gexf.NewGraph()

	g.SetNodeAttrs([]gexf.Attr{
		{Title: "type", Type: gexf.String, Default: "unknown"},
	})

	g.SetEdgeAttrs([]gexf.Attr{
		{Title: "salience", Type: gexf.Float, Default: 0.0},
		{Title: "source", Type: gexf.String, Default: "none"},
	})

	processFile := func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// measure performance
		begin := time.Now()

		// read and unmarshal JSON file
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		var r languagepb.AnalyzeEntitiesResponse
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}

		// fmt.Printf("\n%s\n", p)

		// responses = append(responses, &response)

		// fmt.Printf("\n  Dates\n")
		// for _, e := range r.Entities {
		// 	if e.Type == languagepb.Entity_DATE {
		// 		fmt.Printf("    %f %s\n", e.Salience, e.Name)
		// 	}
		// }

		// fmt.Printf("\n  Locations\n")
		// for _, e := range r.Entities {
		// 	if e.Type == languagepb.Entity_LOCATION {
		// 		fmt.Printf("    %f %s\n", e.Salience, e.Name)
		// 	}
		// }

		fmt.Printf("\n  People\n")
		people := make(map[string]*languagepb.Entity)

		for _, e := range r.Entities {
			if e.Type == languagepb.Entity_PERSON {
				id := g.GetID(e.Name)
				people[id] = e
			}
		}

		for _, e := range people {
			id := g.GetID(e.Name)
			g.AddNode(id, e.Name, []gexf.AttrValue{
				{Title: "type", Value: e.Type.String()},
			})

			// add edge to all other people in same document
			for _, e := range people {
				i := g.GetID(e.Name)
				if i == id {
					continue
				}
				g.AddEdge(id, i, []gexf.AttrValue{
					{Title: "salience", Value: e.Salience},
					{Title: "source", Value: filepath.Base(p)},
				})
			}
			fmt.Printf("    %f %s\n", e.Salience, e.Name)
		}

		// fmt.Printf("\n  Organizations\n")
		// for _, e := range r.Entities {
		// 	if e.Type == languagepb.Entity_ORGANIZATION {
		// 		fmt.Printf("    %f %s\n", e.Salience, e.Name)
		// 	}
		// }

		// fmt.Printf("\n  Events\n")
		// for _, e := range r.Entities {
		// 	if e.Type == languagepb.Entity_EVENT {
		// 		fmt.Printf("    %f %s\n", e.Salience, e.Name)
		// 	}
		// }

		fmt.Printf("%s (%f seconds)\n", p, time.Since(begin).Seconds())
		return nil
	}

	filepath.Walk(path, processFile)

	// Generate GEXF file
	out, err := os.Create(outFilename)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	if gexf.Encode(out, g); err != nil {
		panic(err)
	}

	fmt.Printf("GEXF file generated: %s (%f seconds)\n", outFilename, time.Since(start).Seconds())
}
