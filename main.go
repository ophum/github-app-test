package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v38/github"
)

var (
	appID   int64
	privKey string
)

func init() {
	flag.Int64Var(&appID, "app-id", 0, "-app-id 1234")
	flag.StringVar(&privKey, "privkey", "private-key.pem", "-privkey private-key.pem")
	flag.Parse()
}

func main() {
	r := gin.Default()
	r.POST("/github/events", githubEvents)
	r.GET("/setup", getGithubSetup)
	r.POST("/setup", postGithubSetup)
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
func getGithubSetup(ctx *gin.Context) {
	o, _ := ioutil.ReadAll(ctx.Request.Body)
	log.Println("get setup")
	log.Println(string(o))
}

func postGithubSetup(ctx *gin.Context) {
	o, _ := ioutil.ReadAll(ctx.Request.Body)
	log.Println("post setup")
	log.Println(string(o))
}

func githubEvents(ctx *gin.Context) {
	payload, err := github.ValidatePayload(ctx.Request, nil)
	if err != nil {
		log.Println(err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	webhookEvent, err := github.ParseWebHook(github.WebHookType(ctx.Request), payload)
	if err != nil {
		log.Println(err)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	switch event := webhookEvent.(type) {
	case *github.IssuesEvent:
		if err := githubIssueEvent(ctx.Request.Context(), event); err != nil {
			log.Println(err)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	case *github.PullRequestEvent:
		if err := githubPullRequestEvent(ctx.Request.Context(), event); err != nil {
			log.Println(err)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	case *github.CheckSuiteEvent:
		if err := githubCheckSuiteEvent(ctx.Request.Context(), event); err != nil {
			log.Println(err)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	case *github.CheckRunEvent:
		if err := githubCheckRunEvent(ctx.Request.Context(), event); err != nil {
			log.Println(err)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	default:
		log.Println("default...")
	}
}

func githubIssueEvent(ctx context.Context, event *github.IssuesEvent) error {
	log.Println("github issue event: ", event.GetAction())
	log.Println("github issue event: ", event.GetInstallation().GetID())
	if event.GetAction() != "opened" {
		return nil
	}
	client, err := newGithubClient(event.GetInstallation().GetID())
	if err != nil {
		return err
	}

	repoOwner := event.Repo.GetOwner().GetLogin()
	repo := event.Repo.GetName()

	issue := event.GetIssue()
	issueNumber := issue.GetNumber()
	user := issue.GetUser().GetLogin()

	body := "hello @" + user
	comment := &github.IssueComment{
		Body: &body,
	}
	if _, _, err := client.Issues.CreateComment(ctx, repoOwner, repo, issueNumber, comment); err != nil {
		return err
	}
	return nil
}

func githubPullRequestEvent(ctx context.Context, event *github.PullRequestEvent) error {
	log.Println("github pull request event: ", event.GetAction())
	log.Println("github pull request event: ", event.GetInstallation().GetID())

	if event.GetAction() != "opened" {
		return nil
	}
	client, err := newGithubClient(event.GetInstallation().GetID())
	if err != nil {
		return err
	}

	repoOwner := event.Repo.GetOwner().GetLogin()
	repo := event.Repo.GetName()

	pr := event.GetPullRequest()
	prNumber := pr.GetNumber()
	log.Println(prNumber)
	user := pr.GetUser().GetLogin()
	baseRef := pr.GetBase().GetRef()
	headRef := pr.GetHead().GetRef()

	body := fmt.Sprintf("%s %s %s\n", user, baseRef, headRef)
	comment := &github.IssueComment{
		Body: &body,
	}
	if _, _, err := client.Issues.CreateComment(ctx, repoOwner, repo, prNumber, comment); err != nil {
		return err
	}
	//if _, _, err := client.Checks.CreateCheckSuite(ctx, repoOwner, repo, github.CreateCheckSuiteOptions{
	//	HeadSHA: pr.GetHead().GetSHA(),
	//}); err != nil {
	//	return errors.Wrap(err, "creat echeck suite")
	//}

	return nil
}

func githubCheckSuiteEvent(ctx context.Context, event *github.CheckSuiteEvent) error {
	if event.GetAction() != "requested" && event.GetAction() != "rerequested" {
		return nil
	}
	client, err := newGithubClient(event.GetInstallation().GetID())
	if err != nil {
		return err
	}

	repoOwner := event.Repo.GetOwner().GetLogin()
	repo := event.Repo.GetName()
	client.Checks.CreateCheckRun(ctx, repoOwner, repo, github.CreateCheckRunOptions{
		Name:    "check run!!!",
		HeadSHA: event.GetCheckSuite().GetHeadSHA(),
		Actions: []*github.CheckRunAction{
			{
				Label:       "Next",
				Description: "次へ進みます",
				Identifier:  "next",
			},
		},
	})
	return nil
}

func githubCheckRunEvent(ctx context.Context, event *github.CheckRunEvent) error {
	if event.GetAction() != "requested_action" {
		return nil
	}
	client, err := newGithubClient(event.GetInstallation().GetID())
	if err != nil {
		return err
	}
	repoOwner := event.Repo.GetOwner().GetLogin()
	repo := event.Repo.GetName()
	if event.GetRequestedAction().Identifier == "rerun" {
		if _, _, err := client.Checks.CreateCheckRun(ctx, repoOwner, repo, github.CreateCheckRunOptions{
			Name:    "check rerun!!!",
			HeadSHA: event.GetCheckRun().GetHeadSHA(),
			Actions: []*github.CheckRunAction{
				{
					Label:       "Next",
					Description: "次へ進みます",
					Identifier:  "next",
				},
			},
		}); err != nil {
			return err
		}
		return nil
	}
	if event.GetRequestedAction().Identifier != "next" {
		return nil
	}

	if _, _, err := client.Checks.UpdateCheckRun(ctx, repoOwner, repo, event.GetCheckRun().GetID(), github.UpdateCheckRunOptions{
		Name: *event.GetCheckRun().Name,
		Status: func() *string {
			s := "in_progress"
			return &s
		}(),
	}); err != nil {
		return err
	}

	go func(repoOwner, repo string, id int64) error {
		log.Println("check run event start ", *event.GetCheckRun().Name)
		time.Sleep(10 * time.Second)
		log.Println("check run event sleep done ", *event.GetCheckRun().Name)
		if _, _, err := client.Checks.UpdateCheckRun(context.Background(), repoOwner, repo, id, github.UpdateCheckRunOptions{
			Name: *event.GetCheckRun().Name,
			Status: func() *string {
				s := "completed"
				return &s
			}(),
			Conclusion: func() *string {
				s := "success"
				return &s
			}(),
			Actions: []*github.CheckRunAction{
				{
					Label:       "rerun",
					Description: "もう一度実行します",
					Identifier:  "rerun",
				},
			},
		}); err != nil {
			log.Println("check run event", err.Error())
			return err
		}
		return nil
	}(repoOwner, repo, event.GetCheckRun().GetID())
	return nil
}
func newGithubClient(installationID int64) (*github.Client, error) {
	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, appID, installationID, privKey)
	if err != nil {
		return nil, err
	}
	return github.NewClient(&http.Client{
		Transport: itr,
		Timeout:   5 * time.Second,
	}), nil
}
