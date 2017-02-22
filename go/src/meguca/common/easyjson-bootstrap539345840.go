// +build ignore

// TEMPORARY AUTOGENERATED FILE: easyjson bootstapping code to launch
// the actual generator.

package main

import (
  "fmt"
  "os"

  "github.com/mailru/easyjson/gen"

  pkg "meguca/common"
)

func main() {
  g := gen.NewGenerator("posts_easyjson.go")
  g.SetPkg("common", "meguca/common")
  g.NoStdMarshalers()
  g.Add(pkg.EasyJSON_exporter_Board(nil))
  g.Add(pkg.EasyJSON_exporter_Post(nil))
  g.Add(pkg.EasyJSON_exporter_StandalonePost(nil))
  g.Add(pkg.EasyJSON_exporter_Thread(nil))
  if err := g.Run(os.Stdout); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
