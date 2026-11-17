package workflow

// RunControlled is the compatibility entry point for callers without output persistence.
func RunControlled(d Definition, reused map[string]NodeOutput, before func(*NodeInput, map[string]NodeOutput) error, execute func([]NodeInput) ([]NodeOutput, error)) (map[string]NodeOutput, error) {
	return RunAudited(d, reused, before, nil, execute)
}

func (r *runner) finish(input NodeInput, output NodeOutput) error {
	output.Iteration = input.Iteration
	if !output.Skipped {
		if err := ValidateOutput(input.Node, &output); err != nil {
			return err
		}
	}
	if r.after != nil {
		if err := r.after(input, &output); err != nil {
			return err
		}
	}
	if !output.Skipped {
		if err := ValidateOutput(input.Node, &output); err != nil {
			return err
		}
	}
	r.outputs[input.Node.ID] = output
	return nil
}
