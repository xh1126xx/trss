export namespace translator {
	
	export class Config {
	    name: string;
	    base_url: string;
	    api_key: string;
	    model: string;
	    source_lang: string;
	    target_lang: string;
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.model = source["model"];
	        this.source_lang = source["source_lang"];
	        this.target_lang = source["target_lang"];
	        this.prompt = source["prompt"];
	    }
	}

}

