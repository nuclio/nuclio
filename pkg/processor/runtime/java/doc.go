/* package java implements a Java runtime

To implement a handler you need to write a class the implements
io.nuclio.EventHandler and then build a jar to pass nuclio builder.

The default image is using JDK 8

If you have dependecies in other packages, create a fat/uber Jar with all the
dependencies. We currently do not support maven/gradle/sbt/ant/... builds
*/
package java
