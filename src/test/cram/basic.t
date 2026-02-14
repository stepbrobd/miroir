set up a local bare repo as "origin":

  $ git init --bare --initial-branch=master origin.git > /dev/null 2>&1

seed it with an initial commit:

  $ git clone origin.git seed > /dev/null 2>&1
  $ cd seed
  $ git config user.email "test@test"
  $ git config user.name "test"
  $ echo "hello" > readme.txt
  $ git add readme.txt
  $ git commit -m "init" > /dev/null 2>&1
  $ git push origin master > /dev/null 2>&1
  $ cd ..

create a mirror bare repo:

  $ git init --bare --initial-branch=master mirror.git > /dev/null 2>&1

clone the repo into a managed directory:

  $ mkdir -p repos
  $ git clone origin.git repos/test > /dev/null 2>&1

add mirror as a named remote in the cloned repo:

  $ cd repos/test
  $ git remote add mirror ../../mirror.git
  $ cd ../..

write a miroir config:

  $ cat > config.toml <<EOF
  > [general]
  > home = "$PWD/repos"
  > branch = "master"
  > concurrency = 1
  > 
  > [platform.origin]
  > origin = true
  > domain = "localhost"
  > user = ""
  > access = "ssh"
  > 
  > [platform.mirror]
  > origin = false
  > domain = "localhost"
  > user = ""
  > access = "ssh"
  > 
  > [repo.test]
  > description = "test repo"
  > visibility = "public"
  > archived = false
  > EOF

test exec (filter $ lines and paths):

  $ miroir exec -c config.toml -n test -- cat readme.txt 2>&1 | grep -v '^\$' | grep -v 'Miroir'
  hello

push seed update, then test pull:

  $ cd seed
  $ echo "updated" > readme.txt
  $ git add readme.txt
  $ git commit -m "update" > /dev/null 2>&1
  $ git push origin master > /dev/null 2>&1
  $ cd ..

  $ miroir pull -c config.toml -n test > /dev/null 2>&1

verify the pull brought the update:

  $ cat repos/test/readme.txt
  updated

test push to mirror:

  $ miroir push -c config.toml -n test > /dev/null 2>&1

verify mirror has the commits:

  $ git -C mirror.git log --oneline master | wc -l | tr -d ' '
  2

verify mirror has both commits:

  $ git -C mirror.git log --format=%s master
  update
  init

test exec runs in repo context:

  $ miroir exec -c config.toml -n test -- git rev-parse --show-toplevel 2>&1 | grep -v '^\$' | grep -v 'Miroir' | grep -c 'repos/test'
  1

test exec header is printed:

  $ miroir exec -c config.toml -n test -- true 2>&1 | grep -c 'Miroir :: Repo :: Exec'
  1

test pull header is printed:

  $ miroir pull -c config.toml -n test 2>&1 | grep -c 'Miroir :: Repo :: Pull'
  1

test push header is printed:

  $ miroir push -c config.toml -n test 2>&1 | grep -c 'Miroir :: Repo :: Push'
  1

test that we can make changes and push them through:

  $ cd repos/test
  $ git config user.email "test@test"
  $ git config user.name "test"
  $ echo "changed" > readme.txt
  $ git add readme.txt
  $ git commit -m "local change" > /dev/null 2>&1
  $ cd ../..

  $ miroir push -c config.toml -n test > /dev/null 2>&1

  $ git -C mirror.git log --format=%s master
  local change
  update
  init

  $ git -C origin.git log --format=%s master
  local change
  update
  init
