# Vizzini

[Inconceivable tests!](http://www.imdb.com/character/ch0003791/)

These are "beta" versions of some interesting Inigo-style Diego tests.

## What's In Here

- Under the root directory are tests that exercise the core of Diego through a variety of use-cases.  These run against bosh-lite.  Though they are primarily used to accept stories related to the details of Task and LRP behavior, they're a valuable integration suite for Diego as a whole.  Also, they're fast and can safely be run in parallel.  

- Under `/blackbox` is a stress test that runs against bosh-lite.  It uses [veritas](https://github.com/cloudfoundry-incubator/veritas) and [cf](https://github.com/cloudfoundry/cli) to push apps to a local bosh-lite, scale them up, then scale them down.  It emits goroutine counts, etc. of all the components using `veritas vitals`.

#### Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)
