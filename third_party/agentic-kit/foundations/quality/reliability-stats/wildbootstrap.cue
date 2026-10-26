package reliabilitystats

// #Observation also accepts a separate independent multiplier cluster.
// Omission defaults to task; all trials of a task must resolve to one
// cluster (validated by Go). PairedBootstrapCI ignores this field.
#Observation: {cluster?: string}

// #Interval optionally reports independent multiplier clusters.
#Interval: {
	tasks:     int & >=1
	clusters?: int & >=2 & <=tasks
}

// #WildClusterInterval reports tasks and at least two independent clusters.
#WildClusterInterval: #Interval & {tasks: >=2, clusters: int & >=2 & <=tasks}
