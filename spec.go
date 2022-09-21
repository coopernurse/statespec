package statespec

import (
	"fmt"
	"math/rand"
	"time"
)

// SpecConf contains configuration on how to run a Spec
type SpecConf struct {
	// RNG to pass to Command.Gen during run
	Rand *rand.Rand
	// Number of times to run the spec
	Iterations int
	// Max commands to run per iteration
	MaxCmdPerIter int
}

// Spec defines a stateful specification
// S is the state type for this spec and will be passed
// to commands in the spec and mutated during each iteration
type Spec[S any] struct {
	// Setup is an optional callback function that resets the
	// system under test and does any other required initialization
	// Setup is run once before all iterations
	Setup func() error

	// TearDown is an optional callback function run after all
	// iterations have completed
	TearDown func() error

	// InitState is a REQUIRED callback that is run once at the beginning
	// of each iteration. It should return the initial state of the system
	// for that run
	InitState func() S

	// Commands are the list of Command instances that may be run during
	// an interation. As the iteration runs, a random Command is selected
	// and Gen() is run on it.  If Gen() returns a non-nil CommandFunc,
	// that function is run.
	//
	// State S is passed around between commands as the iteration runs
	// and each Command may mutate the state to track expected effects of that
	// command
	Commands []Command[S]
}

// Command is a single side effecting action against the system under test
type Command[S any] struct {
	// Used in return output to identify the command
	Name string

	// Gen is passed the current state and a RNG. If the Command can run in this
	// state, a CommandFunc is returned. If the Command cannot run, return nil.
	//
	// CommandFunc returns CommandOutput. If CommandOutput.Error is non-nil,
	// the spec is considered violated and execution terminates
	Gen func(state S, rnd *rand.Rand) CommandFunc[S]

	// Verify is an optional function that compares the oldState (before Gen was run)
	// with the newState (after Gen was run). Returns true if newState is valid.
	// If Verify returns false, the spec is considered violated and execution terminates.
	Verify func(oldState S, newState S) bool
}

// CommandFunc is a function that runs against the system under test and returns
// a modified S state and potentially an error
type CommandFunc[S any] func() CommandOutput[S]

// CommandOutput is the result of running CommandFunc
type CommandOutput[S any] struct {
	// NewState is the modified state of the system after running the command
	NewState S

	// Description is a value that describes the command. Usually this is the
	// input that was run, but it can be any value that would be useful in
	// troubleshooting an error
	Description any

	// Error represents any error that occurred during command execution
	// A successful command execution should set this to nil
	// Non nil values terminate execution and indicate the specification was violated
	Error error
}

func (s Spec[S]) Run(conf SpecConf) (int, error) {
	if len(s.Commands) == 0 {
		return 0, fmt.Errorf("spec.Run Commands is empty")
	}
	if s.InitState == nil {
		return 0, fmt.Errorf("spec.InitState cannot be nil")
	}

	if s.Setup != nil {
		err := s.Setup()
		if err != nil {
			return 0, fmt.Errorf("spec.Run Setup error: %w", err)
		}
	}

	rnd := conf.Rand
	if rnd == nil {
		seed := time.Now().UnixNano()
		fmt.Printf("conf.Rand nil - configuring default random with seed: %d\n", seed)
		rnd = rand.New(rand.NewSource(seed))
	}

	iters := conf.Iterations
	if iters < 1 {
		iters = 100
	}

	cmdPerIter := conf.MaxCmdPerIter
	if cmdPerIter < 1 {
		cmdPerIter = 20
	}

	var err error
	// it's possible that no commands will want to run
	// put in a an upper limit on how many commands we'll try before
	// terminating this iteration early
	maxTries := 3 * len(s.Commands)
	for i := 0; i < iters && err == nil; i++ {
		state := s.InitState()
		totalCmdsToRun := rnd.Intn(cmdPerIter) + 1
		cmdRun := 0
		tries := 0
		for cmdRun < totalCmdsToRun && tries < maxTries && err == nil {
			// pick random command from spec and ask it to generate a CommandFunc
			c := s.Commands[rnd.Intn(len(s.Commands))]
			cfunc := c.Gen(state, rnd)

			if cfunc == nil {
				// command declined to run
				tries++
			} else {
				// run command
				out := cfunc()
				if out.Error != nil {
					err = fmt.Errorf("spec.Run failed iter: %d step: %d cmd error - cmd=%s %+v state=%+v err=%v",
						i, cmdRun, c.Name, out.Description, state, out.Error)
				}

				// if command has a verify step, run it
				if c.Verify != nil {
					ok := c.Verify(state, out.NewState)
					if !ok {
						err = fmt.Errorf("spec.Run failed iter: %d step: %d verify false - cmd=%s %+v oldState=%+v newState=%+v",
							i, cmdRun, c.Name, out.Description, state, out.NewState)
					}
				}

				// set state to result of command
				state = out.NewState
				cmdRun++
				tries = 0
			}
		}
	}

	if s.TearDown != nil {
		err2 := s.TearDown()
		if err2 != nil {
			if err == nil {
				// return as error from spec run
				err = fmt.Errorf("spec.Run TearDown error: %w", err)
			} else {
				// already have an error - log TearDown err but return original err to caller
				fmt.Printf("statespec ERROR in TearDown: %v\n", err2)
			}
		}
	}

	return iters, err
}
