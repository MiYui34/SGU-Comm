import { mount } from "svelte";
import Preview from "./Preview.svelte";

const target = document.getElementById("sgu");
if (target) mount(Preview, { target });
