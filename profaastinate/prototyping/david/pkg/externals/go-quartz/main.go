package main

import (
	"context"
	"github.com/reugn/go-quartz/quartz"
	"net/http"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create scheduler
	sched := quartz.NewStdScheduler()

	// async start scheduler
	sched.Start(ctx)

	// create jobs
	cronTrigger, _ := quartz.NewCronTrigger("1/5 * * * * *")
	shellJob := quartz.NewShellJob("ls")

	request, _ := http.NewRequest(http.MethodGet, "https://worldtimeapi.org/api/timezone/utc", nil)
	curlJob := quartz.NewCurlJob(request)

	functionJob := quartz.NewFunctionJob(func(_ context.Context) (int, error) { return 42, nil })

	// register jobs to scheduler
	sched.ScheduleJob(ctx, shellJob, cronTrigger)
	sched.ScheduleJob(ctx, curlJob, quartz.NewSimpleTrigger(time.Second*7))
	sched.ScheduleJob(ctx, functionJob, quartz.NewSimpleTrigger(time.Second*5))

	// stop scheduler
	sched.Stop()

	// wait for all workers to exit
	sched.Wait(ctx)
}
