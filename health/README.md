# Health Monitoring Framework

Main purpose of the package is to provide a framework that gives you an ability to collect and send health data and metrics for configured targets.

Target can be anything. Basically it is some part of your program or maybe a whole program that you can logically separate. Health of the target can be simply in two states: healty or unhealthy and the health monitoring framework provides you an ability to mark your target as healthy or unhealthy in different cases. Additionaly we provide an ability to collect health related metrics. You can collect whatever you wants. For example you can collect error rate and if it is constatly hight it can also say something about target health. Abilities to collect heartbeat or error messages also in place.

## How to use

First of all you need to import the package to your golang project.
At the start of your program you need to configure and initialize health manager. After it you can call a health collector in any part of your program with required parameters and collect any data that you need.

### Configuration

You need to provide values for health manager [config](#config) and define [targets](#target) with all metrics and their types that you want to collect in future. 

```go
// define health monitoring configuration
config := health.NewConfig()
config.CollectionCycle = 60 * time.Second

// define your targets
firstTarget, err := target.New(
    firstTargetID, true,
    []string{someMetricID},
    []string{someCounterID},
    []string{someTotalCounterID},
)
if err != nil {
    // handle error here
}
targets := []*target.Target{
    firstTarget,
}
```

### Initializtion

You need to init a couple of important structs here and start health manager.
At this point you should create your [destination](#destination), [writer](#writer) and [manager](#manager). After it you can add your targets to the manager and start it.

```go
// Define writer and its destination
logDestination := writer.NewLogDestination(log.GetLogger())
writer := writer.New([]writer.Destination{logDestination})

// init health manager
manager := health.NewManager(ctx, config)

// add configured targets
manager.AddTargets(targets)

// start health monitoring
// after this you are safe to call collector in any part of your program
health.FrameworkStart(ctx, config, manager, writer)
```

### Collection

After you done with inititalization you can call collector in any part of your program and collect any health data that you want. You can even send a [message](#message) per target.

```go
collector, err := health.GetCollector()
if err != nil {
    // handle an error here
}

// you can collect heartbeat data within configured collection cycle
go collector.HeartBeat(firstTargetID)
// you can store any float data as a metric info
collector.AddMetricValue(firstTargetID, someMetricID, 35.4)
// you can store counter data per cycle (like number of some method executions)
collector.AddToCounter(firstTargetID, someCounterID, 3)
// or store total counter data (like number of running goroutines)
collector.AddToCounter(firstTargetID, someTotalCounterID, 1)
collector.AddToCounter(firstTargetID, someTotalCounterID, -1)
// you can even send error messages per target
msg := target.NewMessage(
    "Some kind of title for your message",
    err,
    true)
collector.HealthMessage(firstTargetID, msg)
// also you have a control over target health status
collector.ChangeHealth(firstTargetID, false)
```

## Demo

You can see how it works in [health/demo](https://github.com/zenoss/zenoss-go-sdk/health/demo) folder. Simply go there, run `go run .` and see a short demonstration about how can we collect different health data for bus (it is not the main purpose of the framework to collect bus health data but anyway...).

## Structs and Interfaces

Here is a list of all public structs and interfaces, how you can use them and what info you should/can provide as parameters

### Config

Config keeps configuration for whole framework instance. Right now we have these avialable config parameters:

* CollectionCycle - how often we should calculate and flush data to a writer. 30 seconds by default.
* RegistrationOnCollect - whether to allow data collection for unregistered targets. Manager will register such targets automatically. Not recommended to use, it is better to explicitly define all targets. Note: in this case you cannot specify counter as a total counter.
* LogLevel - log level will be applied to zerolog logger during manager.Start call. Available values: trace, debug, info, warn, error, fatal, panic

Note: right now we don't have an ability to update Configuration after you passed it as a parameter to manager constructor.

### Target

Provided by health/target package. Target object keeps data about all its metrics and some additional per target configs.

Target data:
* ID
* Type - just a string. Should help to categorize your targets and can be used to help define target priority in future. You can pass empty string, "default" value will be used then.
* MetricIDs - list of float metric IDs (calculate avg value for each metric every cycle)
* CounterIDs - list of counter IDs (resets to 0 every cycle)
* TotalCounterIDs - list of total counter IDs (will not be reset every cycle)

Target config:
* EnableHeartbeat - whether to enable heartbeat data for this target. If false we will not take care about heartbeat data.

### Destination

Destination object should implement Destination interface (lives in health/writer package). It should have two methods: Register and Push. Register takes [health target](#target) when you add it and makes all required work to prepare destination for future data. Push takes [health object](#target-health) and sends it to the place you want. We have these available destinations:
* LogDestination - it will simply output your health data as a log message on info level.
* ZCDestination - it allows you to push health data directly to the ZING under your tenant. It will push target data as a model and health data as metrics (right now only counters and metrics work). To start use ZCDestination you also need to prepare ZCDestinationConfig. Example how it looks like:
```go
config := &writer.ZCDestinationConfig{
    EndpointConfig: &endpoint.Config{
        APIKey:         "<your-api-key>",
        Address:        "api.zing.soy:443",
        // these parameters are used for compact metrics cache
        MinTTL:         432000,
        MaxTTL:         576000,
        CacheSizeLimit: 0,
    },
    SourceName: "my-system",
    SourceType: "zenoss.example.health",
    Metadata: map[string]string{
        "host": "127.0.0.1",
    },
}
```

### Writer

Writer is responsible for two things:
- listening of calculated health data and sending it using destination.Push method.
- listening of registered targets and sending it using destination.Register method.
You should provided intitalized destination as a parameter in writer constructor (function New). Writer interface requires only one method: Start. This method takes channels with target.Health and target.Target objects as a parameters and is responsible to listen it.

### Manager

Manager is a heart of our framework. During Start call it starts to listend for all comunication channels: Collector->Manager->Writer. Manager has target registry and is responsible to calculate all collected health data once per cycle and send it to writer.
Manager also keeps FrameworkStart function. You should use it as a started for health framework. It initializes communication channels and collector and starts writer and manager.

### Message

Message struct lives in health/target package. It has these fields:
* Summary - should describe shortly what happened with your target.
* Error - if the error is a reason to create a message you can add it here to provide additional context.
* AffectHealth - whether you want this message to affect your target health status and mark it as unhealthy.

### Target Health

Lives in health/targer package. It is a final look of target's health data. Right now it has such fields:
* TargetID
* TargetType
* Healthy - health status of the target
* Heartbeat - object with two values: Enabled, Beats. Enabled shows you whether you described that you want to collect heartbeat info. Beats will be true if we received heartbeat info during last cycle.
* Counters - map of all (default and total) counters with their values
* Metrics - map of all metrics with their values
* Messages - list of all messages that we collected within last cycle
