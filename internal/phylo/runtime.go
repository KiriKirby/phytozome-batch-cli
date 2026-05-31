package phylo

import "context"

type Runtime interface {
	Name() string
	Run(context.Context, RunPlan) (RunResult, error)
}

type RuntimeOptions struct {
	Runtime Runtime
}

func RunPlanWithRuntime(ctx context.Context, plan RunPlan, opts RuntimeOptions) (RunResult, error) {
	runtime := opts.Runtime
	if runtime == nil {
		runtime = MegaPHGORuntime{}
	}
	return runtime.Run(ctx, plan)
}
