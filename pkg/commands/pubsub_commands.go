package commands

import "github.com/shoreweaver/shoredb/pkg/resp"

// PUBLISH channel message
func publish(args []resp.Value, db *Database) resp.Value {
	if len(args) != 2 {
		return resp.Value{Type: resp.SimpleError, Str: "ERR wrong number of arguments for 'publish' command"}
	}

	count := db.PubSub.Publish(args[0].Str, args[1].Str)
	return resp.Value{Type: resp.Integer, Num: count}
}
