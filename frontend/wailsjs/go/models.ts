export namespace gui {
	
	export class DiscoveredDevice {
	    device_id: string;
	    address: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.address = source["address"];
	        this.port = source["port"];
	    }
	}

}

export namespace models {
	
	export class ClipboardEntry {
	    id: string;
	    content_type: string;
	    content_hash: string;
	    payload: number[];
	    source_device: string;
	    timestamp: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new ClipboardEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.content_type = source["content_type"];
	        this.content_hash = source["content_hash"];
	        this.payload = source["payload"];
	        this.source_device = source["source_device"];
	        this.timestamp = source["timestamp"];
	        this.size = source["size"];
	    }
	}
	export class Device {
	    id: string;
	    display_name: string;
	    paired: boolean;
	    last_seen: number;
	    public_key: number[];
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.paired = source["paired"];
	        this.last_seen = source["last_seen"];
	        this.public_key = source["public_key"];
	    }
	}

}

