export namespace main {
	
	export class Note {
	    id: string;
	    date: string;
	    time: string;
	    content: string;
	    completed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.date = source["date"];
	        this.time = source["time"];
	        this.content = source["content"];
	        this.completed = source["completed"];
	    }
	}

}

