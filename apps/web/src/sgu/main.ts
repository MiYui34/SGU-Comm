import { mount } from "svelte";
import { mockState } from "./mock";
import Shell from "./Shell.svelte";
import "./theme.css";

const target = document.getElementById("sgu");
if (target) mount(Shell, { target, props: { snapshot: mockState } });
