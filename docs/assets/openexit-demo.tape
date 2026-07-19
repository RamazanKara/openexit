Output docs/assets/openexit-demo.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 22
Set Framerate 24
Set Padding 0
Set TypingSpeed 22ms
Set CursorBlink false
Set Theme "Catppuccin Mocha"

Hide
Type "PROJECT_ROOT=$(pwd)"
Enter
Type "DEMO_ROOT=$(mktemp -d)"
Enter
Type "cp $PROJECT_ROOT/testdata/datadog/small.json $DEMO_ROOT/datadog.json"
Enter
Type "cd $DEMO_ROOT"
Enter
Type "export PATH=$PROJECT_ROOT/bin:$PATH"
Enter
Type "export PS1='openexit ❯ '"
Enter
Type "clear"
Enter
Show

Sleep 1s
Type "openexit datadog scan --fixture datadog.json"
Enter
Sleep 2s

Type "openexit datadog plan --target grafana-lgtm"
Enter
Sleep 2s

Type "openexit datadog export --out migration/"
Enter
Sleep 2s

Type "find migration/generated -type f | sort"
Enter
Sleep 4s
