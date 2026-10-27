export namespace main {
	
	export class Channel {
	    id: string;
	    name: string;
	    description: string;
	    members: string[];
	    createdBy: string;
	    // Go type: time
	    createdAt: any;
	    isPrivate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Channel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.members = source["members"];
	        this.createdBy = source["createdBy"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.isPrivate = source["isPrivate"];
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
	export class Event {
	    id: string;
	    title: string;
	    description: string;
	    channel: string;
	    createdBy: string;
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    rsvp: Record<string, string>;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.channel = source["channel"];
	        this.createdBy = source["createdBy"];
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.rsvp = source["rsvp"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class LinkPreview {
	    url: string;
	    title: string;
	    description: string;
	    siteName: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.siteName = source["siteName"];
	        this.image = source["image"];
	    }
	}
	export class Message {
	    id: string;
	    user: string;
	    text: string;
	    channel: string;
	    // Go type: time
	    timestamp: any;
	    reactions: Record<string, Array<string>>;
	    isPost: boolean;
	    isEdited?: boolean;
	    isFile?: boolean;
	    fileId?: string;
	    fileName?: string;
	    fileSize?: number;
	    mimeType?: string;
	    replyCount?: number;
	    lastReplyAt?: string;
	    replyUsers?: string[];
	    threadId?: string;
	    isCall?: boolean;
	    callStatus?: string;
	    callDuration?: number;
	    callInitiator?: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.user = source["user"];
	        this.text = source["text"];
	        this.channel = source["channel"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.reactions = source["reactions"];
	        this.isPost = source["isPost"];
	        this.isEdited = source["isEdited"];
	        this.isFile = source["isFile"];
	        this.fileId = source["fileId"];
	        this.fileName = source["fileName"];
	        this.fileSize = source["fileSize"];
	        this.mimeType = source["mimeType"];
	        this.replyCount = source["replyCount"];
	        this.lastReplyAt = source["lastReplyAt"];
	        this.replyUsers = source["replyUsers"];
	        this.threadId = source["threadId"];
	        this.isCall = source["isCall"];
	        this.callStatus = source["callStatus"];
	        this.callDuration = source["callDuration"];
	        this.callInitiator = source["callInitiator"];
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
	export class MessagePage {
	    messages: Message[];
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MessagePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], Message);
	        this.hasMore = source["hasMore"];
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
	export class PinnedMessage {
	    message: Message;
	    pinnedBy: string;
	    // Go type: time
	    pinnedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PinnedMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = this.convertValues(source["message"], Message);
	        this.pinnedBy = source["pinnedBy"];
	        this.pinnedAt = this.convertValues(source["pinnedAt"], null);
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
	export class Thread {
	    parent: Message;
	    replies: Message[];
	
	    static createFrom(source: any = {}) {
	        return new Thread(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parent = this.convertValues(source["parent"], Message);
	        this.replies = this.convertValues(source["replies"], Message);
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
	export class User {
	    id: string;
	    username: string;
	    email: string;
	    isOnline: boolean;
	    status: string;
	    lastSeen?: string;
	    token?: string;
	    provider?: string;
	    providerId?: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.email = source["email"];
	        this.isOnline = source["isOnline"];
	        this.status = source["status"];
	        this.lastSeen = source["lastSeen"];
	        this.token = source["token"];
	        this.provider = source["provider"];
	        this.providerId = source["providerId"];
	    }
	}

}

