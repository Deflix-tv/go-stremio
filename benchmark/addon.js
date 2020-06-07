#!/usr/bin/env node

const { addonBuilder, serveHTTP, publishToCentral } = require('stremio-addon-sdk')

const builder = new addonBuilder({
    id: 'org.myexampleaddon',
    version: '1.0.0',

    name: 'simple example',

    catalogs: [],
    resources: ['stream'],
    types: ['movie'],
    idPrefixes: ['tt']
})

builder.defineStreamHandler(function(args) {
    if (args.type === 'movie' && args.id === 'tt1254207') {
        const stream = { url: 'http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4' }
        return Promise.resolve({ streams: [stream] })
    } else {
	return Promise.resolve({ streams: [] })
    }
})

serveHTTP(builder.getInterface(), { port: 7000 })
