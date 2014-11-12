#Vizzini

![Vizzini](http://i.imgur.com/7CaiErW.png)

Inconceivable tests!

These are "beta" versions of some interesting Inigo-style Diego tests.

## What's In Here

- Under the root directory are tests that excercise the receptor through a variety of use-cases.  These run against bosh-lite.  Though they are primarily used to accept stories related to the receptor, they're a valuable integration suite for Diego as a whole.  Also, they're fast and can be parallelized.  

- Under `/acceptance` are some tests Onsi's written to help accept stories.  They tend to only work with bosh-lite and tend to be very low-level, side-stepping things like CC and etcd and the executor and talking directly to components via NATS or http.

- Under `/blackbox` is a stress test that runs against bosh-lite.  It uses [veritas](https://github.com/cloudfoundry-incubator/veritas) and [cf](https://github.com/cloudfoundry/cli) to push apps to a local bosh-lite, scale them up, then scale them down.  It emits goroutine counts, etc. of all the components using `veritas vitals`.

####Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)