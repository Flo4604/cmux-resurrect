## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🔧 | API Server | fullstack | yes | ~/projects/api-server |
- [x] | 📊 | Dashboard | monitor | yes | ~/projects/dashboard |
- [x] | 🧪 | Test Lab | tdd | no | ~/projects/test-lab |

## Templates

### fullstack
- [x] main terminal: `make dev` (focused)
- [x] split right browser: `http://localhost:3000`
- [x] split down: `lazygit`

### monitor
- [x] main terminal: `htop` (focused)
- [x] split right browser: `http://localhost:9090`
- [x] split down: `watch -n 2 'curl -s localhost:3000/health | jq .'`

### tdd
- [x] main terminal: `vim .` (focused)
- [x] split right: `go test ./... -v -count=1`
- [x] split down: `watch -n 5 'wc -l **/*.go | tail -1'`
