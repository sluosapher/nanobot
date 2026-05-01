go generate ./...

$tag = (git describe --tags --exact-match 2>$null)
if ($LASTEXITCODE -eq 0 -and $tag) {
  $ldflags = "-s -w -X github.com/sluosapher/nanobot/pkg/version.Tag=$tag"
} else {
  $ldflags = "-s -w"
}

go build -ldflags "$ldflags" -o bin/nanobot.exe .
