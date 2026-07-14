export namespace ui {
	
	export class PlaceholderKey {
	    label: string;
	    row: number;
	    col: number;
	
	    static createFrom(source: any = {}) {
	        return new PlaceholderKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.row = source["row"];
	        this.col = source["col"];
	    }
	}
	export class PlaceholderGrid {
	    width: number;
	    height: number;
	    keys: PlaceholderKey[];
	
	    static createFrom(source: any = {}) {
	        return new PlaceholderGrid(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.keys = this.convertValues(source["keys"], PlaceholderKey);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

