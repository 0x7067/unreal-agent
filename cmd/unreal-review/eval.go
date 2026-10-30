package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/review/agent"
)

// errEvalCorpora is returned once flag gates pass. The planted, Martian,
// pr38, and labeled corpora stay in unreal-review and are not linked here.
var errEvalCorpora = errors.New("eval corpora stay in unreal-review; this binary does not run them")

// plantedCaseNames and martianCaseNames keep the pre-execution --cases
// errors identical to unreal-review. They are names only.
var plantedCaseNames = []string{
	"race", "deadlock", "nil-deref", "swallowed-error", "wrap-break", "bounds",
	"sql-injection", "channel-leak", "body-leak", "once-failure", "clean",
}

var martianCaseNames = []string{
	"calcom-8087", "calcom-10600", "calcom-10967", "calcom-22345", "calcom-7232",
	"calcom-8330", "calcom-11059", "calcom-14943", "calcom-14740", "calcom-22532",
	"discourse-graphite-1", "discourse-graphite-2", "discourse-graphite-3",
	"discourse-graphite-4", "discourse-graphite-5", "discourse-graphite-6",
	"discourse-graphite-7", "discourse-graphite-8", "discourse-graphite-9",
	"discourse-graphite-10",
	"grafana-79265", "grafana-103633", "grafana-76186", "grafana-107534",
	"grafana-106778", "grafana-90045", "grafana-80329", "grafana-90939",
	"grafana-94942", "grafana-97529",
	"keycloak-greptile-1", "keycloak-32918", "keycloak-33832", "keycloak-36882",
	"keycloak-36880", "keycloak-37038", "keycloak-37429", "keycloak-37634",
	"keycloak-38446", "keycloak-40940",
	"sentry-greptile-1", "sentry-greptile-2", "sentry-greptile-3", "sentry-67876",
	"sentry-greptile-5", "sentry-93824", "sentry-77754", "sentry-80528",
	"sentry-95633", "sentry-80168",
}

var pr38CaseNames = []string{"pr38"}

var labeledCaseNames = []string{"pr31", "pr25", "pr3"}

type evalFlags struct {
	out, model, thinking, strategy, corpus, profile, only, judgeModel *string
	decompose, asJSON                                                 *bool
	timeout                                                           *time.Duration
	parallel, runs                                                    *int
}

func registerEvalFlags(fs *flag.FlagSet) evalFlags {
	var f evalFlags
	f.out = fs.String("out", "", "directory for eval artifacts (default: a fresh temp dir)")
	f.model = fs.String("model", os.Getenv("UNREAL_HARNESS_LLM_MODEL"), "OpenRouter model id")
	f.thinking = fs.String("thinking-level", "high", "low, medium, high, xhigh, or max")
	f.strategy = fs.String("strategy", "single", "single or focused (four discovery passes plus verification)")
	f.decompose = fs.Bool("decompose", false, "use bounded aggregate scopes for every review case")
	f.timeout = fs.Duration("timeout", 20*time.Minute, "agent timeout per case")
	f.asJSON = fs.Bool("json", false, "print machine-readable JSON instead of a table")
	f.corpus = fs.String("corpus", "planted", "planted, martian, pr38, or labeled")
	f.profile = fs.String("profile", "core", "martian golden categories: core, strict, or all")
	f.parallel = fs.Int("parallel", 8, "martian cases reviewed at once")
	f.only = fs.String("cases", "", "comma-separated case names for the selected corpus (default: all)")
	f.judgeModel = fs.String("judge-model", "anthropic/claude-sonnet-5.5", "OpenRouter model that matches martian findings to golden comments")
	f.runs = fs.Int("runs", 1, "repeat the pr38 or labeled corpus and print each run, the mean, and the union of hits")
	return f
}

func cmdEval(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	f := registerEvalFlags(fs)
	if err := fs.Parse(args); err != nil {
		return successOnHelp(err)
	}
	switch *f.corpus {
	case "planted", "martian", "pr38", "labeled":
	default:
		return fmt.Errorf("%w %q: use planted, martian, pr38, or labeled", ErrUnknownCorpus, *f.corpus)
	}
	if *f.runs < 1 {
		return fmt.Errorf("--runs must be at least 1")
	}
	if *f.runs != 1 && *f.corpus != "pr38" && *f.corpus != "labeled" {
		return fmt.Errorf("--runs applies to the pr38 and labeled corpora")
	}
	if *f.model == "" {
		return fmt.Errorf("set --model or UNREAL_HARNESS_LLM_MODEL")
	}
	selected, err := reviewProvider()
	if err != nil {
		return err
	}
	key, err := reviewAPIKey(selected)
	if err != nil {
		return err
	}
	level, err := agent.SanitizeLevel(*f.thinking)
	if err != nil {
		return err
	}
	if _, err := defaultSessionDir(); err != nil {
		return err
	}
	openRouterAPI := os.Getenv("UNREAL_REVIEW_OPENROUTER_API")
	if _, err := agent.ReviewPipeline(*f.strategy, agent.Harness{
		APIKey: key, ThinkingLevel: level, Timeout: *f.timeout,
		Provider: selected.Name, Getenv: reviewGetenv,
	}); err != nil {
		return err
	}
	switch *f.corpus {
	case "planted":
		_, err = selectNamed(plantedCaseNames, *f.only, "planted", func(name string) string { return name })
	case "martian":
		err = prepareMartianGates(*f.parallel, *f.profile, *f.only, openRouterAPI)
	case "pr38":
		_, err = selectNamed(pr38CaseNames, *f.only, "pr38", func(name string) string { return name })
	case "labeled":
		_, err = selectNamed(labeledCaseNames, *f.only, "labeled", func(name string) string { return name })
	}
	if err != nil {
		return err
	}
	return errEvalCorpora
}

func prepareMartianGates(parallel int, profile, cases, openRouterAPI string) error {
	if parallel < 1 {
		return fmt.Errorf("--parallel must be at least 1")
	}
	switch profile {
	case "strict", "core", "all":
	default:
		return fmt.Errorf("unknown profile %q: use core, strict, or all", profile)
	}
	if _, err := selectNamed(martianCaseNames, cases, "martian", func(name string) string { return name }); err != nil {
		return err
	}
	if _, err := agent.OpenRouterBase(openRouterAPI); err != nil {
		return err
	}
	return nil
}
