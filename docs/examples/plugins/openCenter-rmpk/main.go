package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "os"
    "os/exec"
    "sort"
    "strings"
)

const usage = `openCenter-rmpk

Subcommands:
  ops                 Print available subcommands
  countPods --node N  List pods on node N and images used

Examples:
  openCenter rmpk ops
  openCenter rmpk countPods --node worker-1
`

func main() {
    if len(os.Args) < 2 {
        fmt.Fprint(os.Stderr, usage)
        os.Exit(2)
    }

    switch os.Args[1] {
    case "ops":
        fmt.Println("Available subcommands:")
        fmt.Println("- countPods")
    case "countPods":
        if err := runCountPods(os.Args[2:]); err != nil {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
            os.Exit(1)
        }
    case "-h", "--help", "help":
        fmt.Print(usage)
    default:
        fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
        fmt.Fprint(os.Stderr, usage)
        os.Exit(2)
    }
}

func runCountPods(args []string) error {
    for _, a := range args {
        if a == "-h" || a == "--help" || a == "help" {
            fmt.Println("Usage: openCenter rmpk countPods --node <node-name>")
            fmt.Println("Lists pods scheduled on the given node and their images.")
            return nil
        }
    }
    fs := flag.NewFlagSet("countPods", flag.ContinueOnError)
    fs.SetOutput(new(bytes.Buffer)) // suppress default error output; we handle messages
    node := fs.String("node", "", "Node name to filter pods")
    if err := fs.Parse(args); err != nil {
        return fmt.Errorf("parsing args: %w", err)
    }
    if *node == "" {
        return errors.New("--node is required")
    }

    if _, err := exec.LookPath("kubectl"); err != nil {
        return fmt.Errorf("kubectl not found in PATH: %w", err)
    }

    cmd := exec.Command("kubectl", "get", "pods", "-A", "-o", "json")
    out, err := cmd.Output()
    if err != nil {
        if ee, ok := err.(*exec.ExitError); ok {
            return fmt.Errorf("kubectl failed: %s", strings.TrimSpace(string(ee.Stderr)))
        }
        return fmt.Errorf("running kubectl: %w", err)
    }

    var pl podList
    if err := json.Unmarshal(out, &pl); err != nil {
        return fmt.Errorf("decoding kubectl JSON: %w", err)
    }

    imagesSet := map[string]struct{}{}
    var lines []string
    for _, p := range pl.Items {
        if p.Spec.NodeName != *node {
            continue
        }
        imgs := append([]string{}, p.Spec.Images()...)
        if len(imgs) == 0 {
            imgs = []string{"<none>"}
        }
        for _, im := range imgs {
            if im == "<none>" {
                continue
            }
            imagesSet[im] = struct{}{}
        }
        line := fmt.Sprintf("- %s/%s\n  images: %s", p.Metadata.Namespace, p.Metadata.Name, strings.Join(imgs, ", "))
        lines = append(lines, line)
    }

    fmt.Printf("Pods on node %s:\n", *node)
    if len(lines) == 0 {
        fmt.Println("(none)")
    } else {
        for _, l := range lines {
            fmt.Println(l)
        }
    }

    // Print unique images
    var uniq []string
    for im := range imagesSet {
        uniq = append(uniq, im)
    }
    sort.Strings(uniq)
    fmt.Printf("\nUnique images (%d):\n", len(uniq))
    if len(uniq) == 0 {
        fmt.Println("(none)")
    } else {
        for _, im := range uniq {
            fmt.Printf("- %s\n", im)
        }
    }
    return nil
}

// Minimal JSON structures for kubectl get pods -A -o json
type podList struct {
    Items []pod `json:"items"`
}

type pod struct {
    Metadata struct {
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
    } `json:"metadata"`
    Spec podSpec `json:"spec"`
}

type podSpec struct {
    NodeName       string      `json:"nodeName"`
    Containers     []container `json:"containers"`
    InitContainers []container `json:"initContainers"`
    Ephemeral      []econtainer `json:"ephemeralContainers"`
}

type container struct {
    Image string `json:"image"`
}

type econtainer struct {
    Image string `json:"image"`
}

func (s podSpec) Images() []string {
    imgs := make([]string, 0, len(s.Containers)+len(s.InitContainers)+len(s.Ephemeral))
    for _, c := range s.Containers {
        if c.Image != "" {
            imgs = append(imgs, c.Image)
        }
    }
    for _, c := range s.InitContainers {
        if c.Image != "" {
            imgs = append(imgs, c.Image)
        }
    }
    for _, e := range s.Ephemeral {
        if e.Image != "" {
            imgs = append(imgs, e.Image)
        }
    }
    return imgs
}
