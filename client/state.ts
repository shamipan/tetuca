// Stores the central state of the web application

import { Post, PostCollection } from './posts'
import { getClosestID, timedAggregate } from './util'
import { readIDs, storeID } from './db'
import { send } from './connection'

// Server-wide global configurations
interface Configs {
	captcha: boolean
	mature: boolean // Website intended for mature audiences
	disableUserBoards: boolean
	pruneThreads: boolean
	threadExpiryMin: number
	threadExpiryMax: number
	maxSize: number
	defaultLang: string
	defaultCSS: string
	imageRootOverride: string
	links: { [key: string]: string }
}

// Board-specific configurations
export interface BoardConfigs {
	readOnly: boolean
	textOnly: boolean
	forcedAnon: boolean
	forcedLive: boolean
	rbText: boolean
	pyu: boolean
	title: string
	notice: string
	rules: string
	[index: string]: any
}

// The current state of a board or thread page
export type PageState = {
	catalog: boolean
	catalogMode: number
	thread: number
	lastN: number
	page: number
	board: string
	href: string
	threadFormIsOpen: boolean
}

const tenDays = 10 * 24 * 60 * 60 * 1000

// Configuration passed from the server. Some values can be changed during
// runtime.
export const config: Configs = (window as any).config

// Currently existing boards
export let boards: string[] = (window as any).boards

export let boardConfig: BoardConfigs

// Currently existing background videos
export let bgVideos: string[] = (window as any).bgVideos

// Load initial page state
export const page = read(location.href)

// All posts currently displayed
export const posts = new PostCollection()

// Posts I made in any tab
export let mine: Set<number>

// Posts that the user has already seen or scrolled past
export let seenPosts: Set<number>

// Replies to this user's posts the user has already seen
export let seenReplies: Set<number>

// Explicitly hidden posts and threads
export let hidden: Set<number>

// Debug mode with more verbose logging
export let debug: boolean = /[\?&]debug=true/.test(location.href)

// Read page state by parsing a URL
function read(href: string): PageState {
	const u = new URL(href, location.origin),
		thread = u.pathname.match(/^\/\w+\/(\d+)/),
		page = u.search.match(/[&\?]page=(\d+)/)
	return {
		href,
		board: u.pathname.match(/^\/(\w+)\//)[1],
		lastN: /[&\?]last=100/.test(u.search) ? 100 : 0,
		page: page ? parseInt(page[1]) : 0,
		catalog: /^\/\w+\/catalog/.test(u.pathname),
		catalogMode: /^\/\w+\/catalogMod/.test(u.pathname) ? 1 : 0,
		thread: parseInt(thread && thread[1]) || 0,
		threadFormIsOpen: false,
	} as PageState
}

// Load post number sets for specific threads from the database
export function loadFromDB(...threads: number[]) {
	return Promise.all([
		readIDs("mine", threads).then(ids =>
			mine = new Set(ids)),
		readIDs("seen", threads).then(ids =>
			seenReplies = new Set(ids)),
		readIDs("seenPost", threads).then(ids =>
			seenPosts = new Set(ids)),
		readIDs("hidden", threads).then((ids) =>
			hidden = new Set(ids)),
	]).then(() => {
		receive("mine", mine);
		receive("seen", seenReplies);
		receive("seenPosts", seenPosts);
		receive("hidden", hidden);
	})
}


const channels: Map<string, BroadcastChannel | null> = new Map();

function getChannel(name: string): BroadcastChannel | null {
	if (channels.has(name)) {
		return channels.get(name);
	}

	const newChannel = typeof BroadcastChannel === 'function' ? new BroadcastChannel(name) : null;
	channels.set(name, newChannel);
	return newChannel;
}

// Broadcast to other tabs
function propagate(channel: string, ...data: any[]) {
	const chan = getChannel(channel);
	chan && chan.postMessage(data);
}

// Receive updates from other tabs
function receive(channel: string, store: Set<number>) {
	const chan = getChannel(channel);
	if (chan) {
		chan.onmessage = (e: MessageEvent) => {
			for (const el of e.data) {
				store.add(el);
			}
		};
	}
}

// batch ids and send at most every 200ms to avoid spamming broadcasts
const batchedPropagateSeenPost = timedAggregate<number>(propagate.bind(null, "seenPost"));
const batchedStoreSeenPost = timedAggregate<{ id: number, op: number }>(storeID.bind(null, "seenPost", tenDays));

// Store the ID of a post this client created
export function storeMine(id: number, op: number) {
	mine.add(id);
	propagate("mine", id);
	storeID("mine", tenDays, { id, op });
}

// Store the ID of a post that replied to one of the user's posts
export function storeSeenReply(id: number, op: number) {
	seenReplies.add(id);
	propagate("seen", id);
	storeID("seen", tenDays, { id, op });
}

export function storeSeenPost(id: number, op: number) {
	seenPosts.add(id);
	batchedPropagateSeenPost(id);
	batchedStoreSeenPost({ id, op });
}

// Store the ID of a post or thread to hide
export function storeHidden(id: number, op: number) {
	hidden.add(id);
	propagate("hidden", id);
	storeID("hidden", tenDays * 3 * 6, { id, op });
}

export function setBoardConfig(c: BoardConfigs) {
	boardConfig = c
}

// Retrieve model of closest parent post
export function getModel(el: Element): Post {
	const id = getClosestID(el)
	if (!id) {
		return null
	}
	return PostCollection.getFromAll(id)
}

// Display or hide the loading animation
export function displayLoading(display: boolean) {
	const el = document.getElementById('loading-image')
	if (el) {
		el.style.display = display ? 'block' : 'none'
	}
}

; (window as any).debugMode = () => {
	debug = true;
	(window as any).send = send
}
