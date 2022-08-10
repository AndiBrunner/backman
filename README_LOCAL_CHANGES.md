The following files were adapted after cloning from https://github.com/swisscom/backman

- manifest.yml
Because the docker image was not working (write to S3 failed) the buildback was used.
The buildpack is also necessary because we only want to backup the elasticsearch index from yesterday
diff --git a/manifest.yml b/manifest.yml
--- a/manifest.yml
+++ b/manifest.yml
 
   # ### push either as docker image
-  docker:
-    image: jamesclonk/backman:1.20.1 # choose version from https://hub.docker.com/r/jamesclonk/backman/tags, or 'latest'
+  #docker:
+    #image: jamesclonk/backman:1.20.1 # choose version from https://hub.docker.com/r/jamesclonk/backman/tags, or 'latest'
   # ### or as buildpack/src
-  # buildpacks:
-  # - https://github.com/cloudfoundry/apt-buildpack
-  # - nodejs_buildpack
-  # - go_buildpack
-  # command: backman
-  # path: .
+  buildpacks:
+  - nodejs_buildpack
+  - go_buildpack
+  #- https://github.com/cloudfoundry/apt-buildpack
+  command: backman
+  path: .
 


diff --git a/service/elasticsearch/backup.go b/service/elasticsearch/backup.go
--- a/service/elasticsearch/backup.go
+++ b/service/elasticsearch/backup.go
 
+        yesterdaydate := time.Now().Add(-24*time.Hour).Format("2006.01.02")
+
        // prepare elasticdump command
        var command []string
        command = append(command, "elasticdump")
-       command = append(command, "--quiet")
-       command = append(command, fmt.Sprintf("--input=%s", connectstring))
+       command = append(command, fmt.Sprintf("--input=%s/index-esc-wl-%s", connectstring, yesterdaydate))
        command = append(command, "--output=$")
