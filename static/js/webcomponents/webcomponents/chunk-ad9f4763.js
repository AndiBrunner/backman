/*! Built with http://stenciljs.com */
const{h:t}=window.webcomponents;var e;!function(t){t.setSelectionBatch="SET_SELECTION_BATCH",t.select="SELECT",t.commitSelectionBatch="COMMIT_SELECTION_BATCH",t.setMultiple="SET_MULTIPLE",t.setDirection="SET_DIRECTION",t.setSelect="SET_SELECT",t.setAnimationDuration="SET_ANIMATION_DURATION",t.toggleOptionEl="TOGGLE_OPTION_EL",t.commitOptionElsBatch="COMMIT_OPTION_ELS_BATCH",t.toggleOptGroupEl="TOGGLE_OPTGROUP_EL",t.commitOptGroupElsBatch="COMMIT_OPTGROUP_ELS_BATCH",t.setFilter="SET_FILTER",t.setFilterFunction="SET_FILTER_FUNCTION"}(e||(e={}));const o=(t={},o)=>{switch(o.type){case e.setSelectionBatch:return Object.assign({},t,{selectionBatch:o.optionEls});case e.commitSelectionBatch:let n=t.selectionSorted;return t.selection!==t.selectionBatch&&(n=t.selectionBatch.concat().sort((e,o)=>{const n=t.optionElsSorted.indexOf(e),i=t.optionElsSorted.indexOf(o);return n>i?1:n<i?-1:0})),Object.assign({},t,{selection:t.selectionBatch,selectionSorted:n});case e.select:let i=t.selectionBatch;if(o.optionEl)if(t.multiple){const t=i.indexOf(o.optionEl);i=t>-1||"remove"===o.strategy?i.filter(t=>t!==o.optionEl):i.concat(o.optionEl)}else i[0]===o.optionEl?"remove"===o.strategy&&(i=[]):"add"===o.strategy&&(i=[o.optionEl]);else i.length&&(i=[]);return Object.assign({},t,{selectionBatch:i});case e.setMultiple:return Object.assign({},t,{multiple:o.multiple});case e.setDirection:return Object.assign({},t,{direction:o.direction});case e.setSelect:return Object.assign({},t,{select:o.select});case e.setAnimationDuration:return Object.assign({},t,{animationDuration:o.animationDuration});case e.toggleOptionEl:return-1===t.optionElsBatch.indexOf(o.optionEl)?Object.assign({},t,{optionElsBatch:[...t.optionElsBatch,o.optionEl]}):Object.assign({},t,{optionElsBatch:t.optionElsBatch.filter(t=>t!==o.optionEl)});case e.toggleOptGroupEl:return-1===t.optgroupElsBatch.indexOf(o.optgroupEl)?Object.assign({},t,{optgroupElsBatch:[...t.optgroupElsBatch,o.optgroupEl]}):Object.assign({},t,{optgroupElsBatch:t.optgroupElsBatch.filter(t=>t!==o.optgroupEl)});case e.commitOptionElsBatch:let s=t.optionElsSorted;return t.optionEls!==t.optionElsBatch&&(s=t.optionElsBatch.concat().sort((t,e)=>{const o=t.compareDocumentPosition(e);return o<=Node.DOCUMENT_POSITION_PRECEDING?-1:o<=Node.DOCUMENT_POSITION_FOLLOWING?1:0}).reverse()),Object.assign({},t,{optionEls:t.optionElsBatch,optionElsSorted:s});case e.commitOptGroupElsBatch:return Object.assign({},t,{optgroupEls:t.optgroupElsBatch});case e.setFilter:return Object.assign({},t,{filter:o.filter});case e.setFilterFunction:return Object.assign({},t,{filterFunction:o.filterFunction});default:return t}};export{o as a,e as b};