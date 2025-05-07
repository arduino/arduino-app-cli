export namespace main {
	
	export class FileInfo {
	    Name: string;
	    Path: string;
	    IsDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Path = source["Path"];
	        this.IsDir = source["IsDir"];
	    }
	}

}

