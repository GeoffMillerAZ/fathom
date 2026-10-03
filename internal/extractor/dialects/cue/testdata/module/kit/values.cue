package kit

import v "example.org/vend"

#Panel: P={
	title: string
	query: P.title
}

netmeta: images: {grafana: digest: "x"}
netmeta: config: {host: "h"}

digest: netmeta.images.grafana.digest
out: (#Conv & {in: netmeta.config.host}).result
vx: (v.#V & {x: 1}).x

sql: #"""
	SELECT \#(digest)
	"""#
