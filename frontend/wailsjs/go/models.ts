export namespace icmp {
	
	export class Capabilities {
	    CanSendICMP: boolean;
	    Reason: string;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CanSendICMP = source["CanSendICMP"];
	        this.Reason = source["Reason"];
	    }
	}

}

export namespace main {
	
	export class NetworkAdapter {
	    name: string;
	    ip: string;
	    cidr: string;
	    mac: string;
	    index: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkAdapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.cidr = source["cidr"];
	        this.mac = source["mac"];
	        this.index = source["index"];
	    }
	}

}

export namespace port {
	
	export class PortResult {
	    port: number;
	    state: string;
	    service: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new PortResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.state = source["state"];
	        this.service = source["service"];
	        this.latency = source["latency"];
	    }
	}

}

