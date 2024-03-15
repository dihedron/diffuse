package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/jessevdk/go-flags"
)

func init() {

	const LevelNone = slog.Level(1000)

	options := &slog.HandlerOptions{
		Level:     LevelNone,
		AddSource: true,
	}

	// my-app -> MY_APP_LOG_LEVEL
	level, ok := os.LookupEnv(
		fmt.Sprintf(
			"%s_LOG_LEVEL",
			strings.ReplaceAll(
				strings.ToUpper(
					path.Base(os.Args[0]),
				),
				"-",
				"_",
			),
		),
	)
	if ok {
		switch strings.ToLower(level) {
		case "debug", "dbg", "d", "trace", "trc", "t":
			options.Level = slog.LevelDebug
		case "informational", "info", "inf", "i":
			options.Level = slog.LevelInfo
		case "warning", "warn", "wrn", "w":
			options.Level = slog.LevelWarn
		case "error", "err", "e", "fatal", "ftl", "f":
			options.Level = slog.LevelError
		case "off", "none", "null", "nil", "no", "n":
			options.Level = LevelNone
			return
		}
	}
	handler := slog.NewTextHandler(os.Stderr, options)
	slog.SetDefault(slog.New(handler))
}

func writeMemProfile(filename string, signals <-chan os.Signal) {
	i := 0
	for range signals {
		filename := fmt.Sprintf("%s-%d.memprof", filename, i)
		i++

		slog.Debug("writing memory profile", "filename", filename)
		f, err := os.Create(filename)
		if err != nil {
			slog.Error("error creating memory profile file", "filename", filename, "error", err)
			continue
		}
		pprof.WriteHeapProfile(f)
		if err := f.Close(); err != nil {
			slog.Error("error closing memory profile file", "filename", filename, "error", err)
		}
	}
}

func main() {

	var options struct {
		Version     bool    `short:"v" long:"version" description:"Show version information"`
		AllowOther  bool    `short:"o" long:"allow-other" description:"Mount with -o allowother"`
		ReadOnly    bool    `short:"r" long:"read-only" description:"Mount with -o ro (readonly)"`
		DirectMount bool    `short:"d" long:"direct-mount" description:"Try to call the mount syscall instead of executing fusermount"`
		Strict      bool    `short:"s" long:"strict" description:"Associated with --direct-mount, doesn't fall back to fusermount if mount fails"`
		CpuProfile  *string `short:"c" long:"cpu-profile" description:"Write CPU profile to the given file"`
		MemProfile  *string `short:"m" long:"mem-profile" description:"Write Memory profile to the given file when SIGUSR1 is received"`
		Debug       bool    `short:"D" long:"debug" description:"Print debugging messages"`
		Args        struct {
			Mountpoint string
			Original   string
		} `positional-args:"yes" required:"yes"`
	}

	args, err := flags.Parse(&options)
	if err != nil {
		slog.Error("error parsing command line", "error", err)
		os.Exit(1)
	}

	slog.Debug("args", "len", len(args))
	if options.CpuProfile != nil {
		slog.Info("writing cpu profile", "filename", *options.CpuProfile)
		f, err := os.Create(*options.CpuProfile)
		if err != nil {
			slog.Error("error opening CPU profile output file", "filename", *options.CpuProfile, "error", err)
			os.Exit(3)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if options.MemProfile != nil {
		slog.Info("send SIGUSR1 to dump memory profile", "pid", os.Getpid())
		profile := make(chan os.Signal, 1)
		signal.Notify(profile, syscall.SIGUSR1)
		go writeMemProfile(*options.MemProfile, profile)
	}
	if options.CpuProfile != nil || options.MemProfile != nil {
		slog.Info("you must unmount gracefully, otherwise the profile file(s) will stay empty!")
	}

	slog.Debug("creating loopback root", "original", options.Args.Original)
	loopbackRoot, err := fs.NewLoopbackRoot(options.Args.Original)
	if err != nil {
		slog.Error("error creating loopback filesystem", "original", options.Args.Original, "error", err)
		os.Exit(1)
	}

	sec := time.Second
	opts := &fs.Options{
		// The timeout options are to be compatible with libfuse defaults,
		// making benchmarking easier.
		AttrTimeout:  &sec,
		EntryTimeout: &sec,

		NullPermissions: true, // Leave file permissions on "000" files as-is

		MountOptions: fuse.MountOptions{
			AllowOther:        options.AllowOther,
			Debug:             options.Debug,
			DirectMount:       options.DirectMount,
			DirectMountStrict: options.DirectMount && options.Strict,
			FsName:            options.Args.Original, // First column in "df -T": original dir
			Name:              "diffuse",             // Second column in "df -T" will be shown as "fuse." + Name
		},
	}
	if opts.AllowOther {
		// Make the kernel check file permissions for us
		opts.MountOptions.Options = append(opts.MountOptions.Options, "default_permissions")
	}
	if options.ReadOnly {
		opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	}
	// Enable diagnostics logging
	if options.Debug {
		opts.Logger = log.New(os.Stderr, "", 0)
	}
	server, err := fs.Mount(options.Args.Mountpoint, loopbackRoot, opts)
	if err != nil {
		slog.Error("loopback filesystem mount failed", "original", options.Args.Original, "mountpoint", options.Args.Mountpoint, "error", err)
		os.Exit(1)
	}
	slog.Info("loopback filesystem mounted", "original", options.Args.Original, "mountpoint", options.Args.Mountpoint)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		slog.Info("unmounting filesystem", "original", options.Args.Original, "mountpoint", options.Args.Mountpoint)
		server.Unmount()
		slog.Info("filesystem unmounted", "original", options.Args.Original, "mountpoint", options.Args.Mountpoint)
	}()
	server.Wait()
}
