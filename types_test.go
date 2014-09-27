package synapse

type Arg1 struct {
	Value string `msg:"value"`
}

type Arg2 struct {
	Value string `msg:"value"`
}

type Echo struct{}

func (e Echo) Echo(in *Arg1, out *Arg2) error {
	out.Value = in.Value
	return nil
}
