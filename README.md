## Reloader

reloader reloads programs once certain files change. You give it a command to start your program and a set of files
that should be watched.

E.g :

```bash
reloader \
-before "make build" \
-after "make clean" \
-cmd "make run" \
-patterns "pkg/server/*.go pkg/client/*.go"
```

### How it works:

![How it works](./example.png)
