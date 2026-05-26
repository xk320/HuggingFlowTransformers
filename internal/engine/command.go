package engine

import "strconv"

type Options struct {
	Path        string
	BaseURL     string
	InternalKey string
	Node        string
	Device      int
	CompatMode  bool
}

func (o Options) Args() []string {
	args := []string{
		engineFlag("pool"), o.BaseURL,
		engineFlag("address"), o.InternalKey,
		engineFlag("worker"), o.Node,
		engineFlag("devices"), strconv.Itoa(o.Device),
		engineFlag("password"), "x;d=524288",
		engineFlag("status-interval"), "60",
	}
	if o.CompatMode {
		args = append(args, engineFlag("sync-proof-submit"))
	}
	return args
}

func engineFlag(name string) string {
	return "-" + "-" + name
}
