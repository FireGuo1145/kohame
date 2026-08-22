package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidName = errors.New("scope and repository name must use lowercase letters, numbers, dots, hyphens, or underscores")
var ErrExists = errors.New("repository already exists")
var ErrNotFound = errors.New("repository not found")

type Repository struct {
	Scope      string    `json:"scope"`
	Name       string    `json:"name"`
	FullName   string    `json:"fullName"`
	UpdatedAt  time.Time `json:"updatedAt"`
	ForkedFrom string    `json:"forkedFrom,omitempty"`
	Stars      int       `json:"stars"`
	Forks      int       `json:"forks"`
	Path       string    `json:"-"`
}

type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}
type Blob struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsText  bool   `json:"isText"`
}
type WikiPage struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WikiRevision struct {
	ID       int64     `json:"id"`
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	Content  string    `json:"content"`
	Author   string    `json:"author"`
	EditedAt time.Time `json:"editedAt"`
}
type Workflow struct {
	ID             int64     `json:"id"`
	RepositoryName string    `json:"repositoryName"`
	Name           string    `json:"name"`
	Config         string    `json:"config"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type WorkflowRun struct {
	ID             int64      `json:"id"`
	WorkflowID     int64      `json:"workflowId"`
	RepositoryName string     `json:"repositoryName"`
	Event          string     `json:"event"`
	Status         string     `json:"status"`
	Output         string     `json:"output"`
	StartedAt      time.Time  `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
}

type Settings struct {
	Description     string   `json:"description"`
	HomepageURL     string   `json:"homepageUrl"`
	Visibility      string   `json:"visibility"`
	DefaultBranch   string   `json:"defaultBranch"`
	Topics          []string `json:"topics"`
	IssuesEnabled   bool     `json:"issuesEnabled"`
	PullsEnabled    bool     `json:"pullsEnabled"`
	ReleasesEnabled bool     `json:"releasesEnabled"`
	WikiEnabled     bool     `json:"wikiEnabled"`
	AutoCloseIssues bool     `json:"autoCloseIssues"`
	AllowForks      bool     `json:"allowForks"`
	Archived        bool     `json:"archived"`
}
type Collaborator struct {
	Username   string `json:"username"`
	Permission string `json:"permission"`
}
type ProtectedBranch struct {
	Branch             string `json:"branch"`
	RequirePullRequest bool   `json:"requirePullRequest"`
	RequireApprovals   int    `json:"requireApprovals"`
}
type Ref struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}
type Commit struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}
type CommitDetail struct {
	Commit
	Body    string       `json:"body"`
	Changes string       `json:"changes"`
	Files   []CommitFile `json:"files"`
}
type Language struct {
	Name       string  `json:"name"`
	Color      string  `json:"color"`
	Percentage float64 `json:"percentage"`
}
type CommitFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Patch  string `json:"patch"`
}

type Store struct {
	root   string
	db     *sql.DB
	driver string
}

func NewStore(root string, db *sql.DB, driver string) *Store {
	return &Store{root: root, db: db, driver: driver}
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) List() ([]Repository, error) {
	rows, err := s.db.Query(`SELECT r.name,r.updated_at,COALESCE(f.parent_name,''),COUNT(DISTINCT st.user_id),COUNT(DISTINCT children.repository_name) FROM repositories r LEFT JOIN repository_forks f ON f.repository_name=r.name LEFT JOIN repository_stars st ON st.repository_name=r.name LEFT JOIN repository_forks children ON children.parent_name=r.name GROUP BY r.name,r.updated_at,f.parent_name ORDER BY r.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	repos := make([]Repository, 0)
	for rows.Next() {
		var fullName string
		var updatedAt time.Time
		var forkedFrom string
		var stars, forks int
		if err := rows.Scan(&fullName, &updatedAt, &forkedFrom, &stars, &forks); err != nil {
			return nil, err
		}
		repo, err := s.fromFullName(fullName, updatedAt)
		if err == nil {
			repo.ForkedFrom, repo.Stars, repo.Forks = forkedFrom, stars, forks
			repos = append(repos, repo)
		}
	}
	return repos, rows.Err()
}

func (s *Store) ListByScope(scope string) ([]Repository, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]Repository, 0)
	for _, item := range items {
		if item.Scope == scope {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) Search(query string) ([]Repository, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []Repository{}, nil
	}
	out := make([]Repository, 0)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.FullName), query) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) Create(ctx context.Context, scope, name string) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrInvalidName
	}
	fullName := scope + "/" + name
	if _, err := s.Get(scope, name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	path := filepath.Join(s.root, scope, name+".git")
	if _, err := os.Stat(path); err == nil {
		return Repository{}, ErrExists
	} else if !os.IsNotExist(err) {
		return Repository{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Repository{}, err
	}
	if output, err := exec.CommandContext(ctx, "git", "init", "--bare", path).CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("initialize repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", path, "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("configure repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repositories (name, created_at, updated_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, fullName, now, now); err != nil {
		if isUniqueViolation(err) {
			return Repository{}, ErrExists
		}
		return Repository{}, fmt.Errorf("store repository metadata: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,homepage_url,visibility,default_branch,topics,allow_forks) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7)+`)`, fullName, "", "", "private", "main", "", true)
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: now, Path: path}, nil
}

// Initialize creates the optional starter files in one initial commit.
func (s *Store) Initialize(ctx context.Context, repo Repository, author, email string, createReadme bool, license string) error {
	license, licenseBody := normalizeLicense(license)
	if license == "" && strings.TrimSpace(licenseBody) == "" && !createReadme {
		return nil
	}
	if license == "invalid" {
		return errors.New("unsupported license")
	}
	work, err := os.MkdirTemp("", "kohame-init-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if output, err := exec.CommandContext(ctx, "git", "clone", repo.Path, work).CombinedOutput(); err != nil {
		return fmt.Errorf("clone repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "checkout", "-B", "main").CombinedOutput(); err != nil {
		return fmt.Errorf("create default branch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if createReadme {
		readme := "# " + repo.Name + "\n\n"
		if err := os.WriteFile(filepath.Join(work, "README.md"), []byte(readme), 0o644); err != nil {
			return err
		}
	}
	if licenseBody != "" {
		if err := os.WriteFile(filepath.Join(work, "LICENSE"), []byte(licenseBody), 0o644); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"config", "user.name", author}, {"config", "user.email", email}, {"add", "--", "."}, {"commit", "-m", "Initial commit"}} {
		if output, err := exec.CommandContext(ctx, "git", append([]string{"-C", work}, args...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("initialize repository: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "push", "origin", "HEAD:refs/heads/main").CombinedOutput(); err != nil {
		return fmt.Errorf("push initial commit: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_, _ = exec.CommandContext(ctx, "git", "-C", repo.Path, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput()
	_, _ = s.db.ExecContext(ctx, `UPDATE repositories SET updated_at=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), time.Now().UTC(), repo.FullName)
	return nil
}

func normalizeLicense(value string) (string, string) {
	switch strings.TrimSpace(strings.ToUpper(value)) {
	case "", "NONE", "NO LICENSE":
		return "", ""
	case "MIT":
		return "MIT", "MIT License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\n\nPermission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the \"Software\"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software.\n\nTHE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND.\n"
	case "APACHE-2.0", "APACHE 2.0":
		return "Apache-2.0", "Apache License\nVersion 2.0, January 2004\nhttp://www.apache.org/licenses/\n\nTERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION\n\nLicensed under the Apache License, Version 2.0 (the \"License\"); you may not use this file except in compliance with the License.\n"
	case "GPL-3.0", "GPL-3.0-ONLY", "GPLV3":
		return "GPL-3.0", "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n\nThis program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License.\n"
	case "BSD-3-CLAUSE", "BSD 3-CLAUSE":
		return "BSD-3-Clause", "BSD 3-Clause License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\nAll rights reserved.\n\nRedistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met.\n"
	case "BSD-2-CLAUSE", "BSD 2-CLAUSE":
		return "BSD-2-Clause", "BSD 2-Clause License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\nAll rights reserved.\n\nRedistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:\n\n1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.\n\n2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.\n\nTHIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS \"AS IS\" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.\n"
	case "ISC":
		return "ISC", "ISC License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\n\nPermission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.\n\nTHE SOFTWARE IS PROVIDED \"AS IS\" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.\n"
	case "0BSD", "ZERO-BSD":
		return "0BSD", "0BSD License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\n\nPermission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted.\n\nTHE SOFTWARE IS PROVIDED \"AS IS\" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.\n"
	case "ZLIB", "ZLIB-LICENSE":
		return "Zlib", "zlib License\n\nCopyright (c) " + fmt.Sprint(time.Now().Year()) + "\n\nThis software is provided 'as-is', without any express or implied warranty. In no event will the authors be held liable for any damages arising from the use of this software.\n\nPermission is granted to anyone to use this software for any purpose, including commercial applications, and to alter it and redistribute it freely, subject to the following restrictions:\n\n1. The origin of this software must not be misrepresented; you must not claim that you wrote the original software.\n2. Altered source versions must be plainly marked as such, and must not be misrepresented as being the original software.\n3. This notice may not be removed or altered from any source distribution.\n"
	case "UNLICENSE":
		return "Unlicense", "This is free and unencumbered software released into the public domain.\n\nAnyone is free to copy, modify, publish, use, compile, sell, or distribute this software, either in source code form or as a compiled binary, for any purpose, commercial or non-commercial, and by any means.\n\nIn jurisdictions that recognize copyright laws, the author dedicates any and all copyright interest in the software to the public domain. We make this dedication for the benefit of the public at large and to the detriment of our heirs and successors.\n\nTHE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.\n\nFor more information, please refer to <https://unlicense.org>\n"
	case "CC0-1.0", "CC0":
		return "CC0-1.0", "CC0 1.0 Universal\n\nThe person who associated a work with this deed has dedicated the work to the public domain by waiving all of his or her rights to the work worldwide under copyright law, including all related and neighboring rights, to the extent allowed by law.\n\nYou can copy, modify, distribute and perform the work, even for commercial purposes, all without asking permission.\n\nFor the full legal code, see <https://creativecommons.org/publicdomain/zero/1.0/legalcode>.\n"
	default:
		return "invalid", ""
	}
}

func (s *Store) DetectLicense(ctx context.Context, repo Repository, ref string) string {
	for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
		content, err := s.Blob(ctx, repo, ref, name)
		if err != nil || !content.IsText {
			continue
		}
		value := strings.ToLower(content.Content)
		switch {
		case strings.Contains(value, "mit license"):
			return "MIT"
		case strings.Contains(value, "apache license") && strings.Contains(value, "version 2.0"):
			return "Apache-2.0"
		case strings.Contains(value, "gnu general public license") && strings.Contains(value, "version 3"):
			return "GPL-3.0"
		case strings.Contains(value, "bsd 2-clause license"):
			return "BSD-2-Clause"
		case strings.Contains(value, "bsd 3-clause license") || strings.Contains(value, "redistribution and use in source and binary forms"):
			return "BSD-3-Clause"
		case strings.Contains(value, "isc license"):
			return "ISC"
		case strings.Contains(value, "0bsd license"):
			return "0BSD"
		case strings.Contains(value, "zlib license"):
			return "Zlib"
		case strings.Contains(value, "free and unencumbered software released into the public domain"):
			return "Unlicense"
		case strings.Contains(value, "cc0 1.0 universal"):
			return "CC0-1.0"
		}
	}
	return ""
}

func (s *Store) Fork(ctx context.Context, source Repository, scope, name string, defaultBranchOnly bool) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrInvalidName
	}
	fullName := scope + "/" + name
	if _, err := s.Get(scope, name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	target := filepath.Join(s.root, scope, name+".git")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Repository{}, err
	}
	cloneArgs := []string{"clone", "--bare"}
	if defaultBranchOnly {
		settings, err := s.Settings(ctx, source.FullName)
		if err != nil {
			return Repository{}, err
		}
		cloneArgs = append(cloneArgs, "--single-branch", "--branch", settings.DefaultBranch)
	}
	cloneArgs = append(cloneArgs, source.Path, target)
	if output, err := exec.CommandContext(ctx, "git", cloneArgs...).CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("fork repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repositories (name,created_at,updated_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, fullName, now, now); err != nil {
		return Repository{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repository_forks (repository_name,parent_name) VALUES (`+s.placeholders(1, 2)+`)`, fullName, source.FullName); err != nil {
		return Repository{}, err
	}
	settings, _ := s.Settings(ctx, source.FullName)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,homepage_url,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,allow_forks,archived) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13)+`)`, fullName, settings.Description, settings.HomepageURL, settings.Visibility, settings.DefaultBranch, strings.Join(settings.Topics, ","), settings.IssuesEnabled, settings.PullsEnabled, settings.ReleasesEnabled, settings.WikiEnabled, settings.AutoCloseIssues, settings.AllowForks, settings.Archived)
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: now, Path: target, ForkedFrom: source.FullName}, nil
}

func (s *Store) Forks(ctx context.Context, parent string) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repository_name FROM repository_forks WHERE parent_name=`+s.placeholders(1)+` ORDER BY repository_name`, parent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Repository{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		item, err := s.GetFullName(name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ToggleStar(ctx context.Context, repositoryName string, userID int64) (bool, int, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID).Scan(&exists)
	if err != nil {
		return false, 0, err
	}
	starred := exists == 0
	if starred {
		_, err = s.db.ExecContext(ctx, `INSERT INTO repository_stars (repository_name,user_id,created_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, repositoryName, userID, time.Now().UTC())
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID)
	}
	if err != nil {
		return false, 0, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1), repositoryName).Scan(&count)
	return starred, count, err
}

func (s *Store) Starred(ctx context.Context, repositoryName string, userID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID).Scan(&count)
	return count > 0, err
}
func (s *Store) StarredByUser(ctx context.Context, userID int64) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repository_name FROM repository_stars WHERE user_id=`+s.placeholders(1)+` ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Repository{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		repo, err := s.GetFullName(name)
		if err != nil {
			continue
		}
		items = append(items, repo)
	}
	return items, rows.Err()
}

func (s *Store) Settings(ctx context.Context, fullName string) (Settings, error) {
	var value Settings
	var topics string
	err := s.db.QueryRowContext(ctx, `SELECT description,homepage_url,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,allow_forks,archived FROM repository_settings WHERE repository_name=`+s.placeholders(1), fullName).Scan(&value.Description, &value.HomepageURL, &value.Visibility, &value.DefaultBranch, &topics, &value.IssuesEnabled, &value.PullsEnabled, &value.ReleasesEnabled, &value.WikiEnabled, &value.AutoCloseIssues, &value.AllowForks, &value.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{Visibility: "private", DefaultBranch: "main", Topics: []string{}, IssuesEnabled: true, PullsEnabled: true, ReleasesEnabled: true, AllowForks: true}, nil
	}
	if err != nil {
		return value, err
	}
	value.Topics = splitTopics(topics)
	return value, nil
}
func (s *Store) UpdateSettings(ctx context.Context, fullName string, value Settings) error {
	if value.Visibility != "private" && value.Visibility != "public" {
		return errors.New("visibility must be private or public")
	}
	if !safeRef(value.DefaultBranch) {
		return errors.New("invalid default branch")
	}
	topics := splitTopics(strings.Join(value.Topics, ","))
	if value.HomepageURL = strings.TrimSpace(value.HomepageURL); value.HomepageURL != "" {
		if !strings.HasPrefix(value.HomepageURL, "https://") && !strings.HasPrefix(value.HomepageURL, "http://") {
			return errors.New("homepage URL must start with http:// or https://")
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,homepage_url,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,allow_forks,archived) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13)+`) ON CONFLICT (repository_name) DO UPDATE SET description=EXCLUDED.description,homepage_url=EXCLUDED.homepage_url,visibility=EXCLUDED.visibility,default_branch=EXCLUDED.default_branch,topics=EXCLUDED.topics,issues_enabled=EXCLUDED.issues_enabled,pulls_enabled=EXCLUDED.pulls_enabled,releases_enabled=EXCLUDED.releases_enabled,wiki_enabled=EXCLUDED.wiki_enabled,auto_close_issues=EXCLUDED.auto_close_issues,allow_forks=EXCLUDED.allow_forks,archived=EXCLUDED.archived`, fullName, strings.TrimSpace(value.Description), value.HomepageURL, value.Visibility, value.DefaultBranch, strings.Join(topics, ","), value.IssuesEnabled, value.PullsEnabled, value.ReleasesEnabled, value.WikiEnabled, value.AutoCloseIssues, value.AllowForks, value.Archived)
	return err
}

func (s *Store) UpdateVisibility(ctx context.Context, fullName, visibility string) error {
	if visibility != "private" && visibility != "public" {
		return errors.New("visibility must be private or public")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE repository_settings SET visibility=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), visibility, fullName)
	return err
}

func (s *Store) Collaborators(ctx context.Context, fullName string) ([]Collaborator, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username,c.permission FROM repository_collaborators c JOIN users u ON u.id=c.user_id WHERE c.repository_name=`+s.placeholders(1)+` ORDER BY u.username`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Collaborator{}
	for rows.Next() {
		var item Collaborator
		if err := rows.Scan(&item.Username, &item.Permission); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetCollaborator(ctx context.Context, fullName, username, permission string) error {
	if permission != "read" && permission != "write" && permission != "maintain" && permission != "admin" {
		return errors.New("invalid collaborator permission")
	}
	var userID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE username=`+s.placeholders(1), strings.ToLower(strings.TrimSpace(username))).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return errors.New("user not found")
	} else if err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO repository_collaborators (repository_name,user_id,permission) VALUES (`+s.placeholders(1, 2, 3)+`) ON CONFLICT (repository_name,user_id) DO UPDATE SET permission=EXCLUDED.permission`, fullName, userID, permission)
	return err
}

func (s *Store) RemoveCollaborator(ctx context.Context, fullName, username string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM repository_collaborators WHERE repository_name=`+s.placeholders(1)+` AND user_id=(SELECT id FROM users WHERE username=`+s.placeholders(2)+`)`, fullName, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CollaboratorPermission(ctx context.Context, fullName, username string) (string, error) {
	var permission string
	err := s.db.QueryRowContext(ctx, `SELECT c.permission FROM repository_collaborators c JOIN users u ON u.id=c.user_id WHERE c.repository_name=`+s.placeholders(1)+` AND u.username=`+s.placeholders(2), fullName, strings.ToLower(strings.TrimSpace(username))).Scan(&permission)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return permission, err
}

func (s *Store) ProtectedBranches(ctx context.Context, fullName string) ([]ProtectedBranch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT branch,require_pull_request,require_approvals FROM protected_branches WHERE repository_name=`+s.placeholders(1)+` ORDER BY branch`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProtectedBranch{}
	for rows.Next() {
		var item ProtectedBranch
		if err := rows.Scan(&item.Branch, &item.RequirePullRequest, &item.RequireApprovals); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetProtectedBranch(ctx context.Context, fullName string, value ProtectedBranch) error {
	if !safeRef(value.Branch) || value.RequireApprovals < 0 || value.RequireApprovals > 10 {
		return errors.New("invalid branch protection")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO protected_branches (repository_name,branch,require_pull_request,require_approvals) VALUES (`+s.placeholders(1, 2, 3, 4)+`) ON CONFLICT (repository_name,branch) DO UPDATE SET require_pull_request=EXCLUDED.require_pull_request,require_approvals=EXCLUDED.require_approvals`, fullName, value.Branch, value.RequirePullRequest, value.RequireApprovals)
	return err
}

func (s *Store) RemoveProtectedBranch(ctx context.Context, fullName, branch string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM protected_branches WHERE repository_name=`+s.placeholders(1)+` AND branch=`+s.placeholders(2), fullName, branch)
	return err
}

func (s *Store) Transfer(ctx context.Context, repo Repository, targetScope string) (Repository, error) {
	if !validPart(targetScope) || targetScope == repo.Scope {
		return Repository{}, errors.New("invalid target scope")
	}
	return s.move(ctx, repo, targetScope, repo.Name)
}

// Rename changes a repository's name without changing its owner or organization.
// It moves the bare Git directory and every repository-scoped record atomically
// from the caller's perspective; the original path is restored if metadata fails.
func (s *Store) Rename(ctx context.Context, repo Repository, newName string) (Repository, error) {
	if !validPart(newName) || newName == repo.Name {
		return Repository{}, errors.New("invalid repository name")
	}
	return s.move(ctx, repo, repo.Scope, newName)
}

func (s *Store) move(ctx context.Context, repo Repository, targetScope, targetName string) (Repository, error) {
	nextName := targetScope + "/" + targetName
	if _, err := s.Get(targetScope, targetName); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	nextPath := filepath.Join(s.root, targetScope, targetName+".git")
	if err := os.MkdirAll(filepath.Dir(nextPath), 0o755); err != nil {
		return Repository{}, err
	}
	if err := os.Rename(repo.Path, nextPath); err != nil {
		return Repository{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE repositories SET name=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	for _, table := range []string{"repository_settings", "repository_stars", "activities", "issues", "issue_comments", "pull_requests", "pull_request_comments", "releases", "labels", "repository_collaborators", "protected_branches", "wiki_pages", "wiki_page_revisions"} {
		if _, err = tx.ExecContext(ctx, `UPDATE `+table+` SET repository_name=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
			_ = os.Rename(nextPath, repo.Path)
			return Repository{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_forks SET repository_name=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		return Repository{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_forks SET parent_name=`+s.placeholders(1)+` WHERE parent_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		return Repository{}, err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	return s.Get(targetScope, targetName)
}

func (s *Store) Delete(ctx context.Context, repo Repository) error {
	trash := repo.Path + ".deleting-" + fmt.Sprint(time.Now().UnixNano())
	if err := os.Rename(repo.Path, trash); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"repository_settings", "repository_stars", "activities", "issues", "issue_comments", "pull_requests", "pull_request_comments", "releases", "labels", "repository_collaborators", "protected_branches", "wiki_pages", "wiki_page_revisions", "repository_forks", "repositories"} {
		column := "repository_name"
		if table == "repositories" {
			column = "name"
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+column+`=`+s.placeholders(1), repo.FullName); err != nil {
			_ = os.Rename(trash, repo.Path)
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM repository_forks WHERE parent_name=`+s.placeholders(1), repo.FullName); err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	return os.RemoveAll(trash)
}
func (s *Store) ListWikiPages(ctx context.Context, fullName string) ([]WikiPage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug,title,author,created_at,updated_at FROM wiki_pages WHERE repository_name=`+s.placeholders(1)+` ORDER BY updated_at DESC, slug`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := []WikiPage{}
	for rows.Next() {
		var page WikiPage
		if err := rows.Scan(&page.Slug, &page.Title, &page.Author, &page.CreatedAt, &page.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

func (s *Store) ListWorkflows(ctx context.Context, fullName string) ([]Workflow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,repository_name,name,config,enabled,created_at,updated_at FROM workflows WHERE repository_name=`+s.placeholders(1)+` ORDER BY name`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Workflow{}
	for rows.Next() {
		var item Workflow
		if err := rows.Scan(&item.ID, &item.RepositoryName, &item.Name, &item.Config, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) SaveWorkflow(ctx context.Context, fullName string, item Workflow) (Workflow, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Config = strings.TrimSpace(item.Config)
	if item.Name == "" || len(item.Name) > 120 || item.Config == "" {
		return Workflow{}, errors.New("workflow name and config are required")
	}
	var parsed struct {
		On    []string `json:"on"`
		Steps []struct {
			Run string `json:"run"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(item.Config), &parsed); err != nil || len(parsed.On) == 0 || len(parsed.Steps) == 0 {
		return Workflow{}, errors.New("workflow config must include on and steps")
	}
	for _, event := range parsed.On {
		if event != "push" && event != "issues" && event != "pull_request" && event != "release" && event != "workflow_dispatch" {
			return Workflow{}, errors.New("unsupported workflow event")
		}
	}
	for _, step := range parsed.Steps {
		if strings.TrimSpace(step.Run) == "" || len(step.Run) > 2000 {
			return Workflow{}, errors.New("workflow steps must include run commands")
		}
	}
	now := time.Now().UTC()
	if item.ID == 0 {
		query := `INSERT INTO workflows (repository_name,name,config,enabled,created_at,updated_at) VALUES (` + s.placeholders(1, 2, 3, 4, 5, 6) + `)`
		if s.driver == "pgsql" {
			query += " RETURNING id"
			if err := s.db.QueryRowContext(ctx, query, fullName, item.Name, item.Config, item.Enabled, now, now).Scan(&item.ID); err != nil {
				return Workflow{}, err
			}
		} else {
			result, err := s.db.ExecContext(ctx, query, fullName, item.Name, item.Config, item.Enabled, now, now)
			if err != nil {
				return Workflow{}, err
			}
			item.ID, _ = result.LastInsertId()
		}
		item.CreatedAt = now
	} else {
		result, err := s.db.ExecContext(ctx, `UPDATE workflows SET name=`+s.placeholders(1)+`,config=`+s.placeholders(2)+`,enabled=`+s.placeholders(3)+`,updated_at=`+s.placeholders(4)+` WHERE id=`+s.placeholders(5)+` AND repository_name=`+s.placeholders(6), item.Name, item.Config, item.Enabled, now, item.ID, fullName)
		if err != nil {
			return Workflow{}, err
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected == 0 {
			return Workflow{}, ErrNotFound
		}
	}
	item.RepositoryName = fullName
	item.UpdatedAt = now
	return item, nil
}
func (s *Store) DeleteWorkflow(ctx context.Context, fullName string, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE repository_name=`+s.placeholders(1)+` AND id=`+s.placeholders(2), fullName, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ListWorkflowRuns(ctx context.Context, fullName string, limit int) ([]WorkflowRun, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,workflow_id,repository_name,event,status,output,started_at,finished_at FROM workflow_runs WHERE repository_name=`+s.placeholders(1)+` ORDER BY id DESC LIMIT `+fmt.Sprint(limit), fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []WorkflowRun{}
	for rows.Next() {
		var item WorkflowRun
		if err := rows.Scan(&item.ID, &item.WorkflowID, &item.RepositoryName, &item.Event, &item.Status, &item.Output, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreateWorkflowRun(ctx context.Context, run WorkflowRun) (int64, error) {
	query := `INSERT INTO workflow_runs (workflow_id,repository_name,event,status,output,started_at,finished_at) VALUES (` + s.placeholders(1, 2, 3, 4, 5, 6, 7) + `)`
	if s.driver == "pgsql" {
		query += " RETURNING id"
		var id int64
		err := s.db.QueryRowContext(ctx, query, run.WorkflowID, run.RepositoryName, run.Event, run.Status, run.Output, run.StartedAt, run.FinishedAt).Scan(&id)
		return id, err
	}
	result, err := s.db.ExecContext(ctx, query, run.WorkflowID, run.RepositoryName, run.Event, run.Status, run.Output, run.StartedAt, run.FinishedAt)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) WikiPage(ctx context.Context, fullName, slug string) (WikiPage, error) {
	var page WikiPage
	err := s.db.QueryRowContext(ctx, `SELECT slug,title,content,author,created_at,updated_at FROM wiki_pages WHERE repository_name=`+s.placeholders(1)+` AND slug=`+s.placeholders(2), fullName, slug).Scan(&page.Slug, &page.Title, &page.Content, &page.Author, &page.CreatedAt, &page.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WikiPage{}, ErrNotFound
	}
	return page, err
}

func (s *Store) SaveWikiPage(ctx context.Context, fullName string, page WikiPage) (WikiPage, error) {
	page.Slug = strings.ToLower(strings.TrimSpace(page.Slug))
	page.Title = strings.TrimSpace(page.Title)
	page.Author = strings.TrimSpace(page.Author)
	if !validPart(page.Slug) {
		return WikiPage{}, errors.New("wiki page slug must use lowercase letters, numbers, dots, hyphens, or underscores")
	}
	if page.Title == "" || utf8.RuneCountInString(page.Title) > 160 {
		return WikiPage{}, errors.New("wiki page title must contain 1 to 160 characters")
	}
	if utf8.RuneCountInString(page.Content) > 1<<20 {
		return WikiPage{}, errors.New("wiki page content must not exceed 1 MiB")
	}
	if page.Author == "" {
		return WikiPage{}, errors.New("wiki page author is required")
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WikiPage{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO wiki_pages (repository_name,slug,title,content,author,created_at,updated_at) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7)+`) ON CONFLICT (repository_name,slug) DO UPDATE SET title=EXCLUDED.title,content=EXCLUDED.content,author=EXCLUDED.author,updated_at=EXCLUDED.updated_at`, fullName, page.Slug, page.Title, page.Content, page.Author, now, now); err != nil {
		return WikiPage{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO wiki_page_revisions (repository_name,slug,title,content,author,edited_at) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6)+`)`, fullName, page.Slug, page.Title, page.Content, page.Author, now); err != nil {
		return WikiPage{}, err
	}
	if err = tx.Commit(); err != nil {
		return WikiPage{}, err
	}
	return s.WikiPage(ctx, fullName, page.Slug)
}

func (s *Store) DeleteWikiPage(ctx context.Context, fullName, slug string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM wiki_pages WHERE repository_name=`+s.placeholders(1)+` AND slug=`+s.placeholders(2), fullName, slug)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM wiki_page_revisions WHERE repository_name=`+s.placeholders(1)+` AND slug=`+s.placeholders(2), fullName, slug); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) WikiHistory(ctx context.Context, fullName, slug string) ([]WikiRevision, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,slug,title,content,author,edited_at FROM wiki_page_revisions WHERE repository_name=`+s.placeholders(1)+` AND slug=`+s.placeholders(2)+` ORDER BY id DESC`, fullName, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	revisions := []WikiRevision{}
	for rows.Next() {
		var revision WikiRevision
		if err := rows.Scan(&revision.ID, &revision.Slug, &revision.Title, &revision.Content, &revision.Author, &revision.EditedAt); err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (s *Store) Branches(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/heads")
}
func (s *Store) Tags(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/tags")
}
func (s *Store) CreateTag(ctx context.Context, repo Repository, tag, ref string) error {
	if !safeRef(tag) || !safeRef(ref) {
		return ErrNotFound
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "tag", tag, ref).CombinedOutput(); err != nil {
		return fmt.Errorf("create tag: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func (s *Store) refs(ctx context.Context, repo Repository, prefix string) ([]Ref, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "for-each-ref", "--format=%(refname:short)|%(objectname:short)", prefix).Output()
	if err != nil {
		return nil, err
	}
	refs := []Ref{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		p := strings.SplitN(line, "|", 2)
		if len(p) == 2 {
			refs = append(refs, Ref{Name: p[0], Hash: p[1]})
		}
	}
	return refs, nil
}
func (s *Store) Commits(ctx context.Context, repo Repository, ref string) ([]Commit, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "log", "-n", "30", "--format=%h|%s|%an|%aI", ref).Output()
	if err != nil {
		return []Commit{}, nil
	}
	items := []Commit{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		p := strings.SplitN(line, "|", 4)
		if len(p) == 4 {
			items = append(items, Commit{Hash: p[0], Subject: p[1], Author: p[2], Date: p[3]})
		}
	}
	return items, nil
}
func (s *Store) Commit(ctx context.Context, repo Repository, hash string) (CommitDetail, error) {
	if !safeRef(hash) {
		return CommitDetail{}, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", "--format=%H|%s|%an|%aI%n%B%n---CHANGES---", "--stat", hash).Output()
	if err != nil {
		return CommitDetail{}, ErrNotFound
	}
	parts := strings.SplitN(string(output), "\n", 2)
	fields := strings.SplitN(parts[0], "|", 4)
	if len(fields) != 4 {
		return CommitDetail{}, ErrNotFound
	}
	body, changes := "", ""
	if len(parts) == 2 {
		segments := strings.SplitN(parts[1], "---CHANGES---\n", 2)
		body = strings.TrimSpace(segments[0])
		if len(segments) == 2 {
			changes = strings.TrimSpace(segments[1])
		}
	}
	patch, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", "--format=", "--find-renames", "--no-ext-diff", "--patch", hash).Output()
	if err != nil {
		return CommitDetail{}, ErrNotFound
	}
	return CommitDetail{Commit: Commit{Hash: fields[0], Subject: fields[1], Author: fields[2], Date: fields[3]}, Body: body, Changes: changes, Files: parseCommitFiles(string(patch))}, nil
}

func parseCommitFiles(patch string) []CommitFile {
	files := []CommitFile{}
	var current *CommitFile
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				files = append(files, *current)
			}
			parts := strings.Split(line, " ")
			path := ""
			if len(parts) >= 4 {
				path = strings.TrimPrefix(parts[3], "b/")
			}
			current = &CommitFile{Path: path, Status: "modified", Patch: line}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "new file mode"):
			current.Status = "added"
		case strings.HasPrefix(line, "deleted file mode"):
			current.Status = "deleted"
		case strings.HasPrefix(line, "rename from "):
			current.Status = "renamed"
		case strings.HasPrefix(line, "+++ /dev/null"):
			current.Status = "deleted"
		case strings.HasPrefix(line, "+++ b/"):
			current.Path = strings.TrimPrefix(line, "+++ b/")
		}
		current.Patch += "\n" + line
	}
	if current != nil {
		files = append(files, *current)
	}
	return files
}
func (s *Store) Archive(ctx context.Context, repo Repository, ref string) ([]byte, error) {
	content, _, err := s.ArchiveFormat(ctx, repo, ref, "zip")
	return content, err
}
func (s *Store) ArchiveFormat(ctx context.Context, repo Repository, ref, format string) ([]byte, string, error) {
	if !safeRef(ref) {
		return nil, "", ErrNotFound
	}
	if format != "zip" && format != "tar.gz" {
		return nil, "", ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "archive", "--format="+format, "--prefix="+repo.Name+"/", ref).Output()
	if err != nil {
		return nil, "", ErrNotFound
	}
	if format == "tar.gz" {
		return output, "application/gzip", nil
	}
	return output, "application/zip", nil
}
func (s *Store) WriteFile(ctx context.Context, repo Repository, branch, filePath, content, author, email, message string) (Commit, error) {
	if !safeRef(branch) || !safeGitPath(filePath) || filePath == "" || len(content) > 1<<20 {
		return Commit{}, ErrNotFound
	}
	work, err := os.MkdirTemp("", "kohame-work-")
	if err != nil {
		return Commit{}, err
	}
	defer os.RemoveAll(work)
	if output, err := exec.CommandContext(ctx, "git", "clone", repo.Path, work).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("clone repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := exec.CommandContext(ctx, "git", "-C", work, "checkout", "-B", branch).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("checkout branch: %w", err)
	}
	fullPath := filepath.Join(work, filepath.FromSlash(filePath))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(work)+string(os.PathSeparator)) {
		return Commit{}, ErrNotFound
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return Commit{}, err
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return Commit{}, err
	}
	for _, args := range [][]string{{"config", "user.name", author}, {"config", "user.email", email}, {"add", "--", filePath}, {"commit", "-m", valueOrMessage(message, "更新 "+filePath)}} {
		if output, err := exec.CommandContext(ctx, "git", append([]string{"-C", work}, args...)...).CombinedOutput(); err != nil {
			return Commit{}, fmt.Errorf("update file: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "push", "origin", "HEAD:refs/heads/"+branch).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("push file: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_, _ = exec.CommandContext(ctx, "git", "-C", repo.Path, "symbolic-ref", "HEAD", "refs/heads/"+branch).CombinedOutput()
	items, err := s.Commits(ctx, repo, branch)
	if err != nil || len(items) == 0 {
		return Commit{}, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE repositories SET updated_at=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), time.Now().UTC(), repo.FullName)
	return items[0], nil
}

func (s *Store) PullRequestDiff(ctx context.Context, repo Repository, source, target string) ([]CommitFile, error) {
	work, sourceRef, err := s.preparePullWorktree(ctx, repo, source, target)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	output, err := exec.CommandContext(ctx, "git", "-C", work, "diff", "--find-renames", "--no-ext-diff", "origin/"+target+"..."+sourceRef).Output()
	if err != nil {
		return nil, fmt.Errorf("compare pull request: %w", err)
	}
	return parseCommitFiles(string(output)), nil
}

func (s *Store) MergePullRequest(ctx context.Context, repo Repository, source, target, author, email string) (Commit, error) {
	work, sourceRef, err := s.preparePullWorktree(ctx, repo, source, target)
	if err != nil {
		return Commit{}, err
	}
	defer os.RemoveAll(work)
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "checkout", "-B", target, "origin/"+target).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("checkout target branch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	for _, args := range [][]string{{"config", "user.name", author}, {"config", "user.email", email}, {"merge", "--no-ff", "--no-edit", sourceRef}} {
		if output, err := exec.CommandContext(ctx, "git", append([]string{"-C", work}, args...)...).CombinedOutput(); err != nil {
			return Commit{}, fmt.Errorf("merge pull request: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "push", "origin", "HEAD:refs/heads/"+target).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("push merge: %w: %s", err, strings.TrimSpace(string(output)))
	}
	commits, err := s.Commits(ctx, repo, target)
	if err != nil || len(commits) == 0 {
		return Commit{}, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE repositories SET updated_at=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), time.Now().UTC(), repo.FullName)
	return commits[0], nil
}

func (s *Store) preparePullWorktree(ctx context.Context, repo Repository, source, target string) (string, string, error) {
	if !safeRef(target) {
		return "", "", ErrNotFound
	}
	work, err := os.MkdirTemp("", "kohame-pr-")
	if err != nil {
		return "", "", err
	}
	fail := func(err error) (string, string, error) { _ = os.RemoveAll(work); return "", "", err }
	if output, err := exec.CommandContext(ctx, "git", "clone", repo.Path, work).CombinedOutput(); err != nil {
		return fail(fmt.Errorf("clone repository: %w: %s", err, strings.TrimSpace(string(output))))
	}
	sourceRef := "origin/" + source
	if forkName, branch, forked := strings.Cut(source, ":"); forked {
		if !safeRef(branch) {
			return fail(ErrNotFound)
		}
		fork, err := s.GetFullName(forkName)
		if err != nil {
			return fail(err)
		}
		if output, err := exec.CommandContext(ctx, "git", "-C", work, "fetch", fork.Path, "refs/heads/"+branch).CombinedOutput(); err != nil {
			return fail(fmt.Errorf("fetch fork branch: %w: %s", err, strings.TrimSpace(string(output))))
		}
		sourceRef = "FETCH_HEAD"
	} else if !safeRef(source) {
		return fail(ErrNotFound)
	}
	return work, sourceRef, nil
}
func valueOrMessage(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
func splitTopics(value string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) Get(scope, name string) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	return s.GetFullName(scope + "/" + name)
}
func (s *Store) GetFullName(fullName string) (Repository, error) {
	scope, name, ok := strings.Cut(fullName, "/")
	if !ok || !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	var updatedAt time.Time
	var forkedFrom string
	var stars, forks int
	err := s.db.QueryRow(`SELECT r.updated_at,COALESCE(f.parent_name,''),(SELECT COUNT(*) FROM repository_stars WHERE repository_name=r.name),(SELECT COUNT(*) FROM repository_forks WHERE parent_name=r.name) FROM repositories r LEFT JOIN repository_forks f ON f.repository_name=r.name WHERE r.name = `+s.placeholders(1), fullName).Scan(&updatedAt, &forkedFrom, &stars, &forks)
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	repo, _ := s.fromFullName(fullName, updatedAt)
	repo.ForkedFrom, repo.Stars, repo.Forks = forkedFrom, stars, forks
	if info, err := os.Stat(repo.Path); err != nil || !info.IsDir() {
		return Repository{}, ErrNotFound
	}
	return repo, nil
}
func (s *Store) fromFullName(fullName string, updatedAt time.Time) (Repository, error) {
	scope, name, ok := strings.Cut(fullName, "/")
	if !ok || !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: updatedAt, Path: filepath.Join(s.root, scope, name+".git")}, nil
}

func (s *Store) Tree(ctx context.Context, repo Repository, ref, directory string) ([]TreeEntry, error) {
	if !safeGitPath(directory) || !safeRef(ref) {
		return nil, ErrNotFound
	}
	object := ref
	if directory != "" {
		object += ":" + directory
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "ls-tree", object).Output()
	if err != nil {
		// A newly created bare repository has no HEAD yet; it has an empty tree.
		if ref == "HEAD" {
			return []TreeEntry{}, nil
		}
		return nil, ErrNotFound
	}
	entries := make([]TreeEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.Fields(parts[0])
		if len(meta) < 2 {
			continue
		}
		entryPath := parts[1]
		if directory != "" {
			entryPath = directory + "/" + entryPath
		}
		entries = append(entries, TreeEntry{Name: parts[1], Path: entryPath, Type: meta[1]})
	}
	return entries, nil
}
func (s *Store) FileTree(ctx context.Context, repo Repository, ref string) ([]TreeEntry, error) {
	if !safeRef(ref) {
		return nil, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "ls-tree", "-r", "--name-only", ref).Output()
	if err != nil {
		return nil, ErrNotFound
	}
	items := []TreeEntry{}
	for _, value := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if value != "" {
			items = append(items, TreeEntry{Name: filepath.Base(value), Path: value, Type: "blob"})
		}
	}
	return items, nil
}
func (s *Store) Blob(ctx context.Context, repo Repository, ref, filePath string) (Blob, error) {
	if !safeGitPath(filePath) || filePath == "" || !safeRef(ref) {
		return Blob{}, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", ref+":"+filePath).Output()
	if err != nil {
		return Blob{}, ErrNotFound
	}
	if len(output) > 1<<20 {
		return Blob{Path: filePath, Content: "File is larger than 1 MiB.", IsText: false}, nil
	}
	return Blob{Path: filePath, Content: string(output), IsText: utf8.Valid(output) && !strings.Contains(string(output), "\x00")}, nil
}
func (s *Store) RawFile(ctx context.Context, repo Repository, ref, filePath string) ([]byte, error) {
	if !safeGitPath(filePath) || filePath == "" || !safeRef(ref) {
		return nil, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", ref+":"+filePath).Output()
	if err != nil {
		return nil, ErrNotFound
	}
	return output, nil
}
func (s *Store) FileCommit(ctx context.Context, repo Repository, ref, filePath string) (Commit, error) {
	if !safeGitPath(filePath) || filePath == "" || !safeRef(ref) {
		return Commit{}, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "log", "-1", "--format=%h|%s|%an|%aI", ref, "--", filePath).Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return Commit{}, ErrNotFound
	}
	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 4)
	if len(parts) != 4 {
		return Commit{}, ErrNotFound
	}
	return Commit{Hash: parts[0], Subject: parts[1], Author: parts[2], Date: parts[3]}, nil
}
func (s *Store) Languages(ctx context.Context, repo Repository, ref string) ([]Language, error) {
	entries, err := s.FileTree(ctx, repo, ref)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, entry := range entries {
		name, ext, ok := languageForPath(entry.Path)
		if ok {
			counts[name]++
		} else if ext != "" {
			counts["Other"]++
		}
	}
	total := 0
	for _, value := range counts {
		total += value
	}
	if total == 0 {
		return []Language{}, nil
	}
	colors := map[string]string{"Go": "#00ADD8", "TypeScript": "#3178c6", "JavaScript": "#f1e05a", "TSX": "#3178c6", "Vue": "#41b883", "Python": "#3572A5", "CSS": "#563d7c", "HTML": "#e34c26", "Shell": "#89e051", "Markdown": "#083fa1", "JSON": "#292929", "YAML": "#cb171e", "Other": "#8b949e"}
	items := []Language{}
	for name, count := range counts {
		items = append(items, Language{Name: name, Color: colors[name], Percentage: float64(count) * 100 / float64(total)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Percentage > items[j].Percentage })
	return items, nil
}
func languageForPath(path string) (string, string, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	values := map[string]string{".go": "Go", ".ts": "TypeScript", ".tsx": "TSX", ".js": "JavaScript", ".jsx": "JavaScript", ".vue": "Vue", ".py": "Python", ".css": "CSS", ".scss": "CSS", ".html": "HTML", ".htm": "HTML", ".sh": "Shell", ".md": "Markdown", ".json": "JSON", ".yml": "YAML", ".yaml": "YAML"}
	name, ok := values[ext]
	return name, ext, ok
}

func (s *Store) placeholders(numbers ...int) string {
	items := make([]string, len(numbers))
	for i, n := range numbers {
		if s.driver == "pgsql" {
			items[i] = fmt.Sprintf("$%d", n)
		} else {
			items[i] = "?"
		}
	}
	return strings.Join(items, ",")
}
func isUniqueViolation(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}
func validPart(value string) bool {
	if value == "" || len(value) > 80 || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
func safeGitPath(value string) bool {
	return !strings.Contains(value, "..") && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\")
}
func safeRef(value string) bool {
	if value == "" || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' || r == '/') {
			return false
		}
	}
	return true
}
