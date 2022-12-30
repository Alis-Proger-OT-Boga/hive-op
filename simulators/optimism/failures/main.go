package main

import (
	"context"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	"github.com/stretchr/testify/require"
)

var tests = []*optimism.TestSpec{
	//{Name: "deposit while sequencer is down", Run: depositWhileSequencerDownTest},
}

func main() {
	suite := hivesim.Suite{
		Name: "optimism failures",
		Description: `
Tests various failure cases against a running Optimism node.
`[1:],
	}

	suite.Add(&hivesim.TestSpec{
		Name:        "failures",
		Description: "Tests failure cases.",
		Run:         runAllTests(tests),
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

func runAllTests(tests []*optimism.TestSpec) func(t *hivesim.T) {
	return func(t *hivesim.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		for _, test := range tests {
			t.Run(hivesim.TestSpec{
				Name:        test.Name,
				Description: test.Description,
				Run: func(t *hivesim.T) {
					d := optimism.NewDevnet(t)
					env := &optimism.TestEnv{
						Context: ctx,
						Devnet:  d,
					}
					test.Run(t, env)
					require.NoError(t, d.ShutdownAll(true))
				},
			})
		}
	}
}
