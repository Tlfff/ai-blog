package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var contextTables = map[string][]string{
	"user":         {"users", "user_session_cleanup_tasks"},
	"article":      {"articles", "article_images", "article_view_histories", "article_view_event_inbox", "article_comment_event_inbox", "article_comment_projection", "article_like_event_inbox", "article_like_projection"},
	"comment":      {"comments", "comment_event_outbox", "comment_like_event_inbox", "comment_like_projection", "comment_like_projection_lock"},
	"like":         {"article_likes", "comment_likes", "article_like_event_outbox", "comment_like_event_outbox"},
	"notification": {"notifications"},
	"search":       {},
}

func TestBoundedContextsDoNotImportOtherRepositories(t *testing.T) {
	root := domainRoot(t)
	for contextName := range contextTables {
		contextDir := filepath.Join(root, contextName)
		walkGoFiles(t, contextDir, func(path string, file *ast.File) {
			for _, imported := range file.Imports {
				value, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				for other := range contextTables {
					if other != contextName && strings.Contains(value, "/internal/domain/"+other+"/repo") {
						t.Errorf("%s imports repository owned by %s: %s", path, other, value)
					}
				}
			}
		})
	}
}

func TestApplicationLayerDoesNotDependOnDomainRepositories(t *testing.T) {
	appRoot := filepath.Join(filepath.Dir(domainRoot(t)), "app")
	walkGoFiles(t, appRoot, func(path string, file *ast.File) {
		for _, imported := range file.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(value, "/internal/domain/") && strings.Contains(value, "/repo") {
				t.Errorf("application layer imports domain repository: %s imports %s", path, value)
			}
		}
	})
}

func TestRepositoriesDoNotWriteTablesOwnedByOtherContexts(t *testing.T) {
	root := domainRoot(t)
	for contextName := range contextTables {
		repoDir := filepath.Join(root, contextName, "repo")
		walkGoFiles(t, repoDir, func(path string, file *ast.File) {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isMutationCall(call.Fun) {
					return true
				}
				literals := make([]string, 0)
				ast.Inspect(call, func(child ast.Node) bool {
					literal, ok := child.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						return true
					}
					value, err := strconv.Unquote(literal.Value)
					if err == nil {
						literals = append(literals, strings.ToLower(value))
					}
					return true
				})
				for owner, tables := range contextTables {
					if owner == contextName {
						continue
					}
					for _, table := range tables {
						for _, literal := range literals {
							if containsTableName(literal, table) {
								t.Errorf("%s repository mutation references %s table %q", contextName, owner, table)
							}
						}
					}
				}
				return true
			})
		})
	}
}

func domainRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func walkGoFiles(t *testing.T, root string, visit func(string, *ast.File)) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		visit(path, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isMutationCall(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "Exec", "Insert", "Update", "Delete":
		return true
	default:
		return false
	}
}

func containsTableName(value, table string) bool {
	replacer := strings.NewReplacer("`", " ", ".", " ", ",", " ", "(", " ", ")", " ", "\n", " ", "\t", " ")
	for _, token := range strings.Fields(replacer.Replace(value)) {
		if token == table {
			return true
		}
	}
	return false
}
