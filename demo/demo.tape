Output demo.gif

Require echo

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 680
Set Theme "Dracula"
Set Padding 24
Set TypingSpeed 50ms
Set PlaybackSpeed 0.8

Hide
Type "export PATH='/Users/ivannikolaev/go/bin:/usr/local/bin:/usr/bin:/bin'"
Enter
Sleep 100ms
Type "rm -f seed.yaml seed.sql seed.yaml.bak"
Enter
Sleep 200ms
Show

Ctrl+L
Sleep 300ms

Type "seed-cli init --schema-file schema.sql -o seed.yaml"
Enter
Sleep 1800ms

Type "seed-cli validate -c seed.yaml"
Enter
Sleep 2500ms

Type "seed-cli fix -c seed.yaml"
Enter
Sleep 1800ms

# Issue 1: orders.status — KindMissingFactoryParam
# flowFactoryParam → cascade → Select: ["enter value", "change factory instead"]
Enter
Sleep 500ms
# Input: "Allowed values (comma-separated):"
Type "pending,active,cancelled"
Enter
Sleep 1200ms

# Issue 2: products.name — unresolved, picks first_name (top ranked)
Enter
Sleep 1000ms

# Issue 3: products.sku — unresolved, picks string (only match for sku)
Enter
Sleep 800ms

# Issue 4: users.metadata — jsonb, flowSetJsonShape
Type `{"plan":"pro","score":42,"first_name":"Alice"}`
Enter
Sleep 2500ms
# schema printed: ✓ first_name  ? plan  ✓ score
# factory select for "plan": [string (score 10) [current], enter value manually, skip for now]
Down
Enter
Sleep 600ms
# manual entry prompt
Type "EnumValueStr"
Enter
Sleep 1200ms
# cascade: Select ["enter value", "change factory instead"]
Enter
Sleep 500ms
# Input: "Allowed values (comma-separated):"
Type "free,pro,enterprise"
Enter
Sleep 2000ms

Type "seed-cli generate -c seed.yaml -o seed.sql"
Enter
Sleep 1500ms

Type "/usr/bin/head -18 seed.sql"
Enter
Sleep 3s

Sleep 1s
