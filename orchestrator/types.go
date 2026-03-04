package orchestrator

type DeletePolicy struct {
	Recursive bool
}

type ListPolicy struct {
	Recursive bool
}

type ApplyPolicy struct {
	Force bool
}
