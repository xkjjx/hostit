package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/google/go-github/v74/github"
	"golang.org/x/oauth2"
)

type GithubObjectStorageProviderManager struct {
	repositoryOwner string
	repositoryName  string
	folderName      string
	githubClient    *github.Client
}

func (githubObjectStorageProviderManager *GithubObjectStorageProviderManager) InstantiateClient() error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("GITHUB_TOKEN not set")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	githubObjectStorageProviderManager.githubClient = github.NewClient(tc)
	if githubObjectStorageProviderManager.githubClient == nil {
		return errors.New("error when creating github client")
	}
	// Determine the authenticated user's login to use as repository owner for Git Data API calls
	user, _, err := githubObjectStorageProviderManager.githubClient.Users.Get(context.Background(), "")
	if err != nil || user == nil || user.Login == nil || *user.Login == "" {
		return fmt.Errorf("failed to determine authenticated user login: %w", err)
	}
	githubObjectStorageProviderManager.repositoryOwner = *user.Login
	return nil
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) VerifyNamespace() (bool, error) {
	_, resp, err := githubObjectStorageProviderManager.githubClient.Repositories.Get(context.Background(), githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return true, nil
		}
		return true, fmt.Errorf("error checking repository: %v", err)
	}

	return false, nil
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) CreateStorageInstance() error {
	repoIsPrivate := false
	autoInit := true
	repoDescription := "Hosted through hostit"
	repo := &github.Repository{
		Name:        &githubObjectStorageProviderManager.repositoryName,
		Description: &repoDescription,
		Private:     &repoIsPrivate,
		AutoInit:    &autoInit,
	}
	// NOTE: pass empty org to create under the authenticated user account
	_, _, err := githubObjectStorageProviderManager.githubClient.Repositories.Create(context.Background(), "", repo)

	return err
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) UploadFilesToNewInstance() error {
	// 100 MB
	const maxFileSizeBytes = 100 * 1024 * 1024
	const branchName = "main"

	client := githubObjectStorageProviderManager.githubClient
	if client == nil {
		return errors.New("client not instantiated")
	}
	if githubObjectStorageProviderManager.repositoryOwner == "" {
		return errors.New("repository owner not set; call InstantiateClient first")
	}

	info, err := os.Stat(githubObjectStorageProviderManager.folderName)
	if err != nil {
		return fmt.Errorf("unable to stat folder '%s': %w", githubObjectStorageProviderManager.folderName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", githubObjectStorageProviderManager.folderName)
	}

	ctx := context.Background()

	// Collect files and create blobs
	treeEntries := []*github.TreeEntry{}
	var uploadErrs []error
	hasCNAMEAtRoot := false

	finder := NewUploadFileFinder()
	filesToUpload, err := finder.FindFiles(githubObjectStorageProviderManager.folderName, maxFileSizeBytes)
	if err != nil {
		return err
	}
	for _, repoPath := range filesToUpload {
		fullPath := filepath.Join(githubObjectStorageProviderManager.folderName, filepath.FromSlash(repoPath))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			uploadErrs = append(uploadErrs, fmt.Errorf("failed to read file '%s': %w", fullPath, err))
			continue
		}
		if repoPath == "CNAME" {
			hasCNAMEAtRoot = true
			current := strings.TrimSpace(string(data))
			desired := strings.TrimSpace(githubObjectStorageProviderManager.repositoryName)
			if current != desired {
				fmt.Printf("updating CNAME content from to '%s'", desired)
				data = []byte(desired)
			}
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		content := encoded
		encoding := "base64"
		blob, _, err := client.Git.CreateBlob(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, &github.Blob{
			Content:  &content,
			Encoding: &encoding,
		})
		if err != nil {
			uploadErrs = append(uploadErrs, fmt.Errorf("failed to create blob for '%s': %w", repoPath, err))
			continue
		}

		mode := "100644"
		typeBlob := "blob"
		treeEntries = append(treeEntries, &github.TreeEntry{
			Path: &repoPath,
			Mode: &mode,
			Type: &typeBlob,
			SHA:  blob.SHA,
		})
		log.Printf("Found '%s'", repoPath)
	}
	if len(uploadErrs) > 0 {
		for _, e := range uploadErrs {
			log.Printf("upload error: %v", e)
		}
		return fmt.Errorf("completed with %d errors", len(uploadErrs))
	}

	// Ensure a CNAME file exists at the repository root so GitHub Pages sets the custom domain
	if !hasCNAMEAtRoot {
		cnameContent := []byte(githubObjectStorageProviderManager.repositoryName)
		encoded := base64.StdEncoding.EncodeToString(cnameContent)
		cnameBlobContent := encoded
		cnameBlobEncoding := "base64"
		blob, _, err := client.Git.CreateBlob(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, &github.Blob{
			Content:  &cnameBlobContent,
			Encoding: &cnameBlobEncoding,
		})
		if err != nil {
			return fmt.Errorf("failed to create blob for CNAME: %w", err)
		}
		mode := "100644"
		typeBlob := "blob"
		cnamePath := "CNAME"
		treeEntries = append(treeEntries, &github.TreeEntry{
			Path: &cnamePath,
			Mode: &mode,
			Type: &typeBlob,
			SHA:  blob.SHA,
		})
	}

	// Determine base tree and parents (if branch exists)
	var baseTreeSHA string
	var parents []*github.Commit

	ref, resp, err := client.Git.GetRef(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, "heads/"+branchName)
	if err != nil {
		if resp == nil || resp.StatusCode != 404 {
			return fmt.Errorf("failed to get ref for branch '%s': %w", branchName, err)
		}
	} else {
		parentCommitSHA := ""
		if ref.Object != nil && ref.Object.SHA != nil {
			parentCommitSHA = *ref.Object.SHA
		}
		if parentCommitSHA == "" {
			return errors.New("unexpected empty parent commit SHA on existing branch")
		}
		parentCommit, _, err := client.Git.GetCommit(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, parentCommitSHA)
		if err != nil {
			return fmt.Errorf("failed to get parent commit: %w", err)
		}
		if parentCommit.Tree != nil && parentCommit.Tree.SHA != nil {
			baseTreeSHA = *parentCommit.Tree.SHA
		}
		parents = []*github.Commit{parentCommit}
	}

	tree, _, err := client.Git.CreateTree(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, baseTreeSHA, treeEntries)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	commitMessage := "Github pages content"
	commitInput := &github.Commit{
		Message: &commitMessage,
		Tree:    tree,
		Parents: parents,
	}
	newCommit, _, err := client.Git.CreateCommit(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, commitInput, nil)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Update or create the branch ref to point to the new commit
	if ref != nil && ref.Ref != nil {
		// Update existing ref
		ref.Object.SHA = newCommit.SHA
		refName := "refs/heads/" + branchName
		ref.Ref = &refName
		_, _, err = client.Git.UpdateRef(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, ref, false)
		if err != nil {
			return fmt.Errorf("failed to update ref for branch '%s': %w", branchName, err)
		}
	} else {
		// Create new ref
		newRefName := "refs/heads/" + branchName
		newRef := &github.Reference{
			Ref: &newRefName,
			Object: &github.GitObject{
				SHA: newCommit.SHA,
			},
		}
		_, _, err = client.Git.CreateRef(ctx, githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, newRef)
		if err != nil {
			return fmt.Errorf("failed to create ref for branch '%s': %w", branchName, err)
		}
	}
	return nil
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) CreateAvailableDomain() error {
	branchName := "main"
	path := "/"
	pages := &github.Pages{
		Source: &github.PagesSource{
			Branch: &branchName,
			Path:   &path,
		},
	}
	_, _, err := githubObjectStorageProviderManager.githubClient.Repositories.EnablePages(
		context.Background(), githubObjectStorageProviderManager.repositoryOwner, githubObjectStorageProviderManager.repositoryName, pages,
	)
	if err != nil {
		return fmt.Errorf("failed to enable Pages: %w", err)
	}
	return nil
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) GetRequiredDnsRecords() ([]*types.ResourceRecordSet, error) {
	return []*types.ResourceRecordSet{
		{
			Name: aws.String(githubObjectStorageProviderManager.repositoryName + "."),
			Type: types.RRTypeCname,
			TTL:  aws.Int64(300),
			ResourceRecords: []types.ResourceRecord{
				{
					Value: aws.String(githubObjectStorageProviderManager.repositoryOwner + ".github.io"),
				},
			},
		},
	}, nil
}

func (githubObjectStorageProviderManager GithubObjectStorageProviderManager) FinalizeHttps() error {
	return nil
}

func NewGithubObjectStorageProviderManager(repositoryName string, folderName string) (*GithubObjectStorageProviderManager, error) {
	if folderName == "" {
		return nil, errors.New("folderName must not be empty")
	}
	return &GithubObjectStorageProviderManager{
		repositoryOwner: "",
		repositoryName:  repositoryName,
		folderName:      folderName,
		githubClient:    nil,
	}, nil
}
