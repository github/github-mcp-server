// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestClassroomService_GetAssignment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/assignments/12", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 12,
			"public_repo": false,
			"title": "Intro to Binaries",
			"type": "individual",
			"invite_link": "https://example.com/a/Lx7jiUgx",
			"invitations_enabled": true,
			"slug": "intro-to-binaries",
			"students_are_repo_admins": false,
			"feedback_pull_requests_enabled": true,
			"max_teams": 0,
			"max_members": 0,
			"editor": "codespaces",
			"accepted": 100,
			"submitted": 40,
			"passing": 10,
			"language": "ruby",
			"deadline": `+referenceTimeStr+`,
			"starter_code_repository": {
				"id": 1296269,
				"full_name": "octocat/Hello-World",
				"html_url": "https://example.com/octocat/Hello-World",
				"node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
				"private": false,
				"default_branch": "main"
			},
			"classroom": {
				"id": 1296269,
				"name": "Programming Elixir",
				"archived": false,
				"url": "https://example.com/classrooms/programming"
			}
		}`)
	})

	ctx := t.Context()
	assignment, _, err := client.Classroom.GetAssignment(ctx, 12)
	if err != nil {
		t.Errorf("Classroom.GetAssignment returned error: %v", err)
	}

	want := &ClassroomAssignment{
		ID:                          Ptr(int64(12)),
		PublicRepo:                  Ptr(false),
		Title:                       Ptr("Intro to Binaries"),
		Type:                        Ptr("individual"),
		InviteLink:                  Ptr("https://example.com/a/Lx7jiUgx"),
		InvitationsEnabled:          Ptr(true),
		Slug:                        Ptr("intro-to-binaries"),
		StudentsAreRepoAdmins:       Ptr(false),
		FeedbackPullRequestsEnabled: Ptr(true),
		MaxTeams:                    Ptr(0),
		MaxMembers:                  Ptr(0),
		Editor:                      Ptr("codespaces"),
		Accepted:                    Ptr(100),
		Submitted:                   Ptr(40),
		Passing:                     Ptr(10),
		Language:                    Ptr("ruby"),
		Deadline:                    &referenceTimestamp,
		StarterCodeRepository: &Repository{
			ID:            Ptr(int64(1296269)),
			FullName:      Ptr("octocat/Hello-World"),
			HTMLURL:       Ptr("https://example.com/octocat/Hello-World"),
			NodeID:        Ptr("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
			Private:       Ptr(false),
			DefaultBranch: Ptr("main"),
		},
		Classroom: &Classroom{
			ID:       Ptr(int64(1296269)),
			Name:     Ptr("Programming Elixir"),
			Archived: Ptr(false),
			URL:      Ptr("https://example.com/classrooms/programming"),
		},
	}

	if !cmp.Equal(assignment, want) {
		t.Errorf("Classroom.GetAssignment returned %+v, want %+v", assignment, want)
	}

	const methodName = "GetAssignment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Classroom.GetAssignment(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.GetAssignment(ctx, 12)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestClassroomService_GetClassroom(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/classrooms/1296269", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 1296269,
			"name": "Programming Elixir",
			"archived": false,
			"organization": {
				"id": 1,
				"login": "programming-elixir",
				"node_id": "MDEyOk9yZ2FuaXphdGlvbjE=",
				"html_url": "https://example.com/programming-elixir",
				"name": "Learn how to build fault tolerant applications",
				"avatar_url": "https://example.com/avatars/u/9919?v=4"
			},
			"url": "https://example.com/classrooms/programming"
		}`)
	})

	ctx := t.Context()
	classroom, _, err := client.Classroom.GetClassroom(ctx, 1296269)
	if err != nil {
		t.Errorf("Classroom.GetClassroom returned error: %v", err)
	}

	want := &Classroom{
		ID:       Ptr(int64(1296269)),
		Name:     Ptr("Programming Elixir"),
		Archived: Ptr(false),
		Organization: &Organization{
			ID:        Ptr(int64(1)),
			Login:     Ptr("programming-elixir"),
			NodeID:    Ptr("MDEyOk9yZ2FuaXphdGlvbjE="),
			HTMLURL:   Ptr("https://example.com/programming-elixir"),
			Name:      Ptr("Learn how to build fault tolerant applications"),
			AvatarURL: Ptr("https://example.com/avatars/u/9919?v=4"),
		},
		URL: Ptr("https://example.com/classrooms/programming"),
	}

	if !cmp.Equal(classroom, want) {
		t.Errorf("Classroom.GetClassroom returned %+v, want %+v", classroom, want)
	}

	const methodName = "GetClassroom"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Classroom.GetClassroom(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.GetClassroom(ctx, 1296269)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestClassroomService_ListClassrooms(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/classrooms", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2", "per_page": "2"})
		fmt.Fprint(w, `[
			{
				"id": 1296269,
				"name": "Programming Elixir",
				"archived": false,
				"url": "https://example.com/classrooms/programming"
			},
			{
				"id": 1296270,
				"name": "Advanced Programming",
				"archived": true,
				"url": "https://example.com/classrooms/2-advanced-programming"
			}
		]`)
	})

	opt := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	classrooms, _, err := client.Classroom.ListClassrooms(ctx, opt)
	if err != nil {
		t.Errorf("Classroom.ListClassrooms returned error: %v", err)
	}

	want := []*Classroom{
		{
			ID:       Ptr(int64(1296269)),
			Name:     Ptr("Programming Elixir"),
			Archived: Ptr(false),
			URL:      Ptr("https://example.com/classrooms/programming"),
		},
		{
			ID:       Ptr(int64(1296270)),
			Name:     Ptr("Advanced Programming"),
			Archived: Ptr(true),
			URL:      Ptr("https://example.com/classrooms/2-advanced-programming"),
		},
	}

	if !cmp.Equal(classrooms, want) {
		t.Errorf("Classroom.ListClassrooms returned %+v, want %+v", classrooms, want)
	}

	const methodName = "ListClassrooms"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.ListClassrooms(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestClassroomService_ListClassroomAssignments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/classrooms/1296269/assignments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2", "per_page": "2"})
		fmt.Fprint(w, `[
			{
				"id": 12,
				"public_repo": false,
				"title": "Intro to Binaries",
				"type": "individual",
				"invite_link": "https://example.com/a/Lx7jiUgx",
				"invitations_enabled": true,
				"slug": "intro-to-binaries",
				"students_are_repo_admins": false,
				"feedback_pull_requests_enabled": true,
				"max_teams": 0,
				"max_members": 0,
				"editor": "codespaces",
				"accepted": 100,
				"submitted": 40,
				"passing": 10,
				"language": "ruby",
				"deadline": `+referenceTimeStr+`,
				"classroom": {
					"id": 1296269,
					"name": "Programming Elixir",
					"archived": false,
					"url": "https://example.com/classrooms/programming"
				}
			},
			{
				"id": 13,
				"public_repo": true,
				"title": "Advanced Programming",
				"type": "group",
				"invite_link": "https://example.com/a/AdvancedProg",
				"invitations_enabled": true,
				"slug": "advanced-programming",
				"students_are_repo_admins": true,
				"feedback_pull_requests_enabled": false,
				"max_teams": 5,
				"max_members": 3,
				"editor": "vscode",
				"accepted": 50,
				"submitted": 25,
				"passing": 20,
				"language": "python",
				"deadline": `+referenceTimeStr+`,
				"classroom": {
					"id": 1296269,
					"name": "Programming Elixir",
					"archived": false,
					"url": "https://example.com/classrooms/programming"
				}
			}
		]`)
	})

	opt := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	assignments, _, err := client.Classroom.ListClassroomAssignments(ctx, 1296269, opt)
	if err != nil {
		t.Errorf("Classroom.ListClassroomAssignments returned error: %v", err)
	}

	want := []*ClassroomAssignment{
		{
			ID:                          Ptr(int64(12)),
			PublicRepo:                  Ptr(false),
			Title:                       Ptr("Intro to Binaries"),
			Type:                        Ptr("individual"),
			InviteLink:                  Ptr("https://example.com/a/Lx7jiUgx"),
			InvitationsEnabled:          Ptr(true),
			Slug:                        Ptr("intro-to-binaries"),
			StudentsAreRepoAdmins:       Ptr(false),
			FeedbackPullRequestsEnabled: Ptr(true),
			MaxTeams:                    Ptr(0),
			MaxMembers:                  Ptr(0),
			Editor:                      Ptr("codespaces"),
			Accepted:                    Ptr(100),
			Submitted:                   Ptr(40),
			Passing:                     Ptr(10),
			Language:                    Ptr("ruby"),
			Deadline:                    &referenceTimestamp,
			Classroom: &Classroom{
				ID:       Ptr(int64(1296269)),
				Name:     Ptr("Programming Elixir"),
				Archived: Ptr(false),
				URL:      Ptr("https://example.com/classrooms/programming"),
			},
		},
		{
			ID:                          Ptr(int64(13)),
			PublicRepo:                  Ptr(true),
			Title:                       Ptr("Advanced Programming"),
			Type:                        Ptr("group"),
			InviteLink:                  Ptr("https://example.com/a/AdvancedProg"),
			InvitationsEnabled:          Ptr(true),
			Slug:                        Ptr("advanced-programming"),
			StudentsAreRepoAdmins:       Ptr(true),
			FeedbackPullRequestsEnabled: Ptr(false),
			MaxTeams:                    Ptr(5),
			MaxMembers:                  Ptr(3),
			Editor:                      Ptr("vscode"),
			Accepted:                    Ptr(50),
			Submitted:                   Ptr(25),
			Passing:                     Ptr(20),
			Language:                    Ptr("python"),
			Deadline:                    &referenceTimestamp,
			Classroom: &Classroom{
				ID:       Ptr(int64(1296269)),
				Name:     Ptr("Programming Elixir"),
				Archived: Ptr(false),
				URL:      Ptr("https://example.com/classrooms/programming"),
			},
		},
	}

	if !cmp.Equal(assignments, want) {
		t.Errorf("Classroom.ListClassroomAssignments returned %+v, want %+v", assignments, want)
	}

	const methodName = "ListClassroomAssignments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Classroom.ListClassroomAssignments(ctx, -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.ListClassroomAssignments(ctx, 1296269, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestClassroomService_ListAcceptedAssignments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/assignments/12/accepted_assignments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2", "per_page": "2"})
		fmt.Fprint(w, `[
			{
				"id": 42,
				"submitted": true,
				"passing": true,
				"commit_count": 5,
				"grade": "10/10",
				"students": [
					{
						"id": 1,
						"login": "octocat",
						"avatar_url": "https://github.com/images/error/octocat_happy.gif",
						"html_url": "https://github.com/octocat"
					}
				],
				"repository": {
					"id": 1296269,
					"full_name": "octocat/Hello-World",
					"html_url": "https://github.com/octocat/Hello-World",
					"node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
					"private": false,
					"default_branch": "main"
				},
				"assignment": {
					"id": 12,
					"public_repo": false,
					"title": "Intro to Binaries",
					"type": "individual",
					"invite_link": "https://example.com/a/Lx7jiUgx",
					"invitations_enabled": true,
					"slug": "intro-to-binaries",
					"students_are_repo_admins": false,
					"feedback_pull_requests_enabled": true,
					"max_teams": 0,
					"max_members": 0,
					"editor": "codespaces",
					"accepted": 100,
					"submitted": 40,
					"passing": 10,
					"language": "ruby",
					"deadline": `+referenceTimeStr+`,
					"classroom": {
						"id": 1296269,
						"name": "Programming Elixir",
						"archived": false,
						"url": "https://example.com/classrooms/programming"
					}
				}
			},
			{
				"id": 43,
				"submitted": false,
				"passing": false,
				"commit_count": 2,
				"grade": "5/10",
				"students": [
					{
						"id": 2,
						"login": "monalisa",
						"avatar_url": "https://github.com/images/error/monalisa_happy.gif",
						"html_url": "https://github.com/monalisa"
					}
				],
				"repository": {
					"id": 1296270,
					"full_name": "monalisa/Hello-World",
					"html_url": "https://github.com/monalisa/Hello-World",
					"node_id": "MDEwOlJlcG9zaXRvcnkxMjk2Mjcw",
					"private": true,
					"default_branch": "main"
				},
				"assignment": {
					"id": 12,
					"public_repo": false,
					"title": "Intro to Binaries",
					"type": "individual",
					"invite_link": "https://example.com/a/Lx7jiUgx",
					"invitations_enabled": true,
					"slug": "intro-to-binaries",
					"students_are_repo_admins": false,
					"feedback_pull_requests_enabled": true,
					"max_teams": 0,
					"max_members": 0,
					"editor": "codespaces",
					"accepted": 100,
					"submitted": 40,
					"passing": 10,
					"language": "ruby",
					"deadline": `+referenceTimeStr+`,
					"classroom": {
						"id": 1296269,
						"name": "Programming Elixir",
						"archived": false,
						"url": "https://example.com/classrooms/programming"
					}
				}
			}
		]`)
	})

	ctx := t.Context()
	opt := &ListOptions{Page: 2, PerPage: 2}
	acceptedAssignments, _, err := client.Classroom.ListAcceptedAssignments(ctx, 12, opt)
	if err != nil {
		t.Errorf("Classroom.ListAcceptedAssignments returned error: %v", err)
	}

	want := []*AcceptedAssignment{
		{
			ID:          Ptr(int64(42)),
			Submitted:   Ptr(true),
			Passing:     Ptr(true),
			CommitCount: Ptr(5),
			Grade:       Ptr("10/10"),
			Students: []*ClassroomUser{
				{
					ID:        Ptr(int64(1)),
					Login:     Ptr("octocat"),
					AvatarURL: Ptr("https://github.com/images/error/octocat_happy.gif"),
					HTMLURL:   Ptr("https://github.com/octocat"),
				},
			},
			Repository: &Repository{
				ID:            Ptr(int64(1296269)),
				FullName:      Ptr("octocat/Hello-World"),
				HTMLURL:       Ptr("https://github.com/octocat/Hello-World"),
				NodeID:        Ptr("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
				Private:       Ptr(false),
				DefaultBranch: Ptr("main"),
			},
			Assignment: &ClassroomAssignment{
				ID:                          Ptr(int64(12)),
				PublicRepo:                  Ptr(false),
				Title:                       Ptr("Intro to Binaries"),
				Type:                        Ptr("individual"),
				InviteLink:                  Ptr("https://example.com/a/Lx7jiUgx"),
				InvitationsEnabled:          Ptr(true),
				Slug:                        Ptr("intro-to-binaries"),
				StudentsAreRepoAdmins:       Ptr(false),
				FeedbackPullRequestsEnabled: Ptr(true),
				MaxTeams:                    Ptr(0),
				MaxMembers:                  Ptr(0),
				Editor:                      Ptr("codespaces"),
				Accepted:                    Ptr(100),
				Submitted:                   Ptr(40),
				Passing:                     Ptr(10),
				Language:                    Ptr("ruby"),
				Deadline:                    &referenceTimestamp,
				Classroom: &Classroom{
					ID:       Ptr(int64(1296269)),
					Name:     Ptr("Programming Elixir"),
					Archived: Ptr(false),
					URL:      Ptr("https://example.com/classrooms/programming"),
				},
			},
		},
		{
			ID:          Ptr(int64(43)),
			Submitted:   Ptr(false),
			Passing:     Ptr(false),
			CommitCount: Ptr(2),
			Grade:       Ptr("5/10"),
			Students: []*ClassroomUser{
				{
					ID:        Ptr(int64(2)),
					Login:     Ptr("monalisa"),
					AvatarURL: Ptr("https://github.com/images/error/monalisa_happy.gif"),
					HTMLURL:   Ptr("https://github.com/monalisa"),
				},
			},
			Repository: &Repository{
				ID:            Ptr(int64(1296270)),
				FullName:      Ptr("monalisa/Hello-World"),
				HTMLURL:       Ptr("https://github.com/monalisa/Hello-World"),
				NodeID:        Ptr("MDEwOlJlcG9zaXRvcnkxMjk2Mjcw"),
				Private:       Ptr(true),
				DefaultBranch: Ptr("main"),
			},
			Assignment: &ClassroomAssignment{
				ID:                          Ptr(int64(12)),
				PublicRepo:                  Ptr(false),
				Title:                       Ptr("Intro to Binaries"),
				Type:                        Ptr("individual"),
				InviteLink:                  Ptr("https://example.com/a/Lx7jiUgx"),
				InvitationsEnabled:          Ptr(true),
				Slug:                        Ptr("intro-to-binaries"),
				StudentsAreRepoAdmins:       Ptr(false),
				FeedbackPullRequestsEnabled: Ptr(true),
				MaxTeams:                    Ptr(0),
				MaxMembers:                  Ptr(0),
				Editor:                      Ptr("codespaces"),
				Accepted:                    Ptr(100),
				Submitted:                   Ptr(40),
				Passing:                     Ptr(10),
				Language:                    Ptr("ruby"),
				Deadline:                    &referenceTimestamp,
				Classroom: &Classroom{
					ID:       Ptr(int64(1296269)),
					Name:     Ptr("Programming Elixir"),
					Archived: Ptr(false),
					URL:      Ptr("https://example.com/classrooms/programming"),
				},
			},
		},
	}

	if !cmp.Equal(acceptedAssignments, want) {
		t.Errorf("Classroom.ListAcceptedAssignments returned %+v, want %+v", acceptedAssignments, want)
	}

	const methodName = "ListAcceptedAssignments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Classroom.ListAcceptedAssignments(ctx, -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.ListAcceptedAssignments(ctx, 12, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestClassroomService_GetAssignmentGrades(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/assignments/12/grades", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
				"assignment_name": "Intro to Binaries",
				"assignment_url": "https://classroom.github.com/assignments/12",
				"starter_code_url": "https://github.com/octocat/Hello-World",
				"github_username": "octocat",
				"roster_identifier": "student123",
				"student_repository_name": "octocat/intro-to-binaries",
				"student_repository_url": "https://github.com/octocat/intro-to-binaries",
				"submission_timestamp": `+referenceTimeStr+`,
				"points_awarded": 10,
				"points_available": 10,
				"group_name": "Team Alpha"
			},
			{
				"assignment_name": "Intro to Binaries",
				"assignment_url": "https://classroom.github.com/assignments/12",
				"starter_code_url": "https://github.com/octocat/Hello-World",
				"github_username": "monalisa",
				"roster_identifier": "student456",
				"student_repository_name": "monalisa/intro-to-binaries",
				"student_repository_url": "https://github.com/monalisa/intro-to-binaries",
				"submission_timestamp": `+referenceTimeStr+`,
				"points_awarded": 8,
				"points_available": 10,
				"group_name": "Team Beta"
			}
		]`)
	})

	ctx := t.Context()
	grades, _, err := client.Classroom.GetAssignmentGrades(ctx, 12)
	if err != nil {
		t.Errorf("Classroom.GetAssignmentGrades returned error: %v", err)
	}

	want := []*AssignmentGrade{
		{
			AssignmentName:        Ptr("Intro to Binaries"),
			AssignmentURL:         Ptr("https://classroom.github.com/assignments/12"),
			StarterCodeURL:        Ptr("https://github.com/octocat/Hello-World"),
			GithubUsername:        Ptr("octocat"),
			RosterIdentifier:      Ptr("student123"),
			StudentRepositoryName: Ptr("octocat/intro-to-binaries"),
			StudentRepositoryURL:  Ptr("https://github.com/octocat/intro-to-binaries"),
			SubmissionTimestamp:   &referenceTimestamp,
			PointsAwarded:         Ptr(10),
			PointsAvailable:       Ptr(10),
			GroupName:             Ptr("Team Alpha"),
		},
		{
			AssignmentName:        Ptr("Intro to Binaries"),
			AssignmentURL:         Ptr("https://classroom.github.com/assignments/12"),
			StarterCodeURL:        Ptr("https://github.com/octocat/Hello-World"),
			GithubUsername:        Ptr("monalisa"),
			RosterIdentifier:      Ptr("student456"),
			StudentRepositoryName: Ptr("monalisa/intro-to-binaries"),
			StudentRepositoryURL:  Ptr("https://github.com/monalisa/intro-to-binaries"),
			SubmissionTimestamp:   &referenceTimestamp,
			PointsAwarded:         Ptr(8),
			PointsAvailable:       Ptr(10),
			GroupName:             Ptr("Team Beta"),
		},
	}

	if !cmp.Equal(grades, want) {
		t.Errorf("Classroom.GetAssignmentGrades returned %+v, want %+v", grades, want)
	}

	const methodName = "GetAssignmentGrades"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Classroom.GetAssignmentGrades(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Classroom.GetAssignmentGrades(ctx, 12)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
